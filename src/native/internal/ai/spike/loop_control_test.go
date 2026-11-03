package spike

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCancelWhileTheModelStallsEndsAborted(t *testing.T) {
	baseline := runtime.NumGoroutine()
	model := NewMockModel(MockTurn{Chunks: textChunks("partial"), Stall: true})
	agent := NewAgent(model, newTestRegistry(t), Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := agent.Run(ctx, &Session{}, UserMessage{Text: "stream"})
	var got []Event
	for event := range events {
		got = append(got, event)
		if event.Kind == EventDelta {
			cancel()
		}
	}

	assertSequence(t, summarize(got), []string{"start", "delta:partial", "aborted"})
	if countTerminal(got) != 1 {
		t.Fatalf("terminal events = %d, want exactly 1", countTerminal(got))
	}
	if closes := model.Closes(); closes != 1 {
		t.Fatalf("model stream closed %d times, want 1", closes)
	}
	waitForGoroutines(t, baseline)
}

func TestCancelDuringAToolAnswersEveryCallAndEndsAborted(t *testing.T) {
	baseline := runtime.NumGoroutine()
	started := make(chan struct{}, 1)
	slow := Tool{
		Spec: ToolSpec{Name: "slow", Description: "blocks until cancelled"},
		Run: func(ctx context.Context, _ json.RawMessage) (string, error) {
			started <- struct{}{}
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	model := NewMockModel(MockTurn{Chunks: concatChunks(
		callChunks(0, "c1", "slow", `{}`),
		callChunks(1, "c2", "slow", `{}`),
	)})
	agent := NewAgent(model, newTestRegistry(t, slow), Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := &Session{}
	events := agent.Run(ctx, session, UserMessage{Text: "run both"})
	var got []Event
	for event := range events {
		got = append(got, event)
		if event.Kind == EventToolStart {
			<-started
			cancel()
		}
	}

	assertSequence(t, summarize(got), []string{"start", "tool_start:slow", "aborted"})
	if text := toolAnswerText(session.Messages, "c1"); text != abortedByCallerText {
		t.Fatalf("answer for c1 = %q, want the cancelled-call text", text)
	}
	if text := toolAnswerText(session.Messages, "c2"); text != abortedByCallerText {
		t.Fatalf("answer for c2 = %q, want the cancelled-call text", text)
	}
	waitForGoroutines(t, baseline)
}

func TestCancelBeforeTheTurnStartsCallsNoModel(t *testing.T) {
	model := NewMockModel(MockTurn{Chunks: textChunks("never")})
	agent := NewAgent(model, newTestRegistry(t), Options{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := summarize(drain(agent.Run(ctx, &Session{}, UserMessage{Text: "hi"})))

	assertSequence(t, got, []string{"start", "aborted"})
	if n := len(model.Requests()); n != 0 {
		t.Fatalf("model received %d requests after cancellation, want 0", n)
	}
}

func TestToolTimeoutBecomesAnErrorOutcomeTheModelCanSee(t *testing.T) {
	waits := Tool{
		Spec:    ToolSpec{Name: "wait", Description: "waits for its context"},
		Timeout: time.Millisecond,
		Run: func(ctx context.Context, _ json.RawMessage) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	model := NewMockModel(
		MockTurn{Chunks: callChunks(0, "c1", "wait", `{}`)},
		MockTurn{Chunks: textChunks("Timed out")},
	)
	agent := NewAgent(model, newTestRegistry(t, waits), Options{})

	session := &Session{}
	got := summarize(drain(agent.Run(context.Background(), session, UserMessage{Text: "wait"})))

	assertSequence(t, got, []string{"start", "tool_start:wait", "tool_end:wait:error", "delta:Timed out", "done"})
	if text := toolAnswerText(session.Messages, "c1"); text != "tool timed out after 1ms" {
		t.Fatalf("timeout answer = %q", text)
	}
}

func TestToolOutputIsCappedOnARuneBoundary(t *testing.T) {
	big := strings.Repeat("é", 100)
	model := NewMockModel(
		MockTurn{Chunks: callChunks(0, "c1", "dump", `{}`)},
		MockTurn{Chunks: textChunks("ok")},
	)
	agent := NewAgent(model, newTestRegistry(t, constantTool("dump", big)), Options{MaxToolOutputBytes: 51})

	session := &Session{}
	drain(agent.Run(context.Background(), session, UserMessage{Text: "dump"}))

	text := toolAnswerText(session.Messages, "c1")
	if !utf8.ValidString(text) {
		t.Fatalf("capped output is not valid UTF-8: %q", text)
	}
	if !strings.Contains(text, "[output truncated: kept 50 of 200 bytes]") {
		t.Fatalf("capped output lacks the truncation note: %q", text)
	}
}

func TestSecondTurnOnABusySessionIsRefused(t *testing.T) {
	model := NewMockModel(MockTurn{Chunks: textChunks("unused")})
	agent := NewAgent(model, newTestRegistry(t), Options{})
	session := &Session{}
	if !session.acquire() {
		t.Fatal("fresh session should be free")
	}
	defer session.release()

	got := drain(agent.Run(context.Background(), session, UserMessage{Text: "again"}))

	assertSequence(t, summarize(got), []string{"error:session_busy"})
	if n := len(model.Requests()); n != 0 {
		t.Fatalf("busy session reached the model %d times", n)
	}
}

func TestSessionKeepsTheTranscriptAcrossTurns(t *testing.T) {
	model := NewMockModel(
		MockTurn{Chunks: textChunks("one")},
		MockTurn{Chunks: textChunks("two")},
	)
	agent := NewAgent(model, newTestRegistry(t), Options{})
	session := &Session{}

	drain(agent.Run(context.Background(), session, UserMessage{Text: "first"}))
	drain(agent.Run(context.Background(), session, UserMessage{Text: "second"}))

	requests := model.Requests()
	if n := len(requests[1].Messages); n != 3 {
		t.Fatalf("second request carries %d messages, want user, assistant, user", n)
	}
	if got := session.Messages[len(session.Messages)-1].Text; got != "two" {
		t.Fatalf("last transcript message = %q, want the second reply", got)
	}
}

func TestUnknownToolIsRefusedWithoutRunning(t *testing.T) {
	runs := 0
	model := NewMockModel(
		MockTurn{Chunks: callChunks(0, "c1", "bash", `{"command":"id"}`)},
		MockTurn{Chunks: textChunks("ok")},
	)
	agent := NewAgent(model, newTestRegistry(t, countingTool("read", "x", &runs)), Options{})

	session := &Session{}
	got := summarize(drain(agent.Run(context.Background(), session, UserMessage{Text: "run"})))

	assertSequence(t, got, []string{"start", "tool_start:bash", "tool_end:bash:error", "delta:ok", "done"})
	if runs != 0 {
		t.Fatalf("registered tool ran %d times for an unknown call", runs)
	}
	if text := toolAnswerText(session.Messages, "c1"); text != "bash is not a registered tool" {
		t.Fatalf("unknown tool answer = %q", text)
	}
}

func TestNameAllowlistRefusesUnlistedToolsBeforeTheyRun(t *testing.T) {
	runs := 0
	model := NewMockModel(
		MockTurn{Chunks: concatChunks(
			callChunks(0, "c1", "read", `{}`),
			callChunks(1, "c2", "fetch", `{}`),
		)},
		MockTurn{Chunks: textChunks("done")},
	)
	registry := newTestRegistry(t, countingTool("read", "ok", &runs), countingTool("fetch", "net", &runs))
	agent := NewAgent(model, registry, Options{Policy: AllowNames("read")})

	session := &Session{}
	got := summarize(drain(agent.Run(context.Background(), session, UserMessage{Text: "both"})))

	assertSequence(t, got, []string{
		"start",
		"tool_start:read", "tool_end:read:ok",
		"tool_start:fetch", "tool_end:fetch:error",
		"delta:done", "done",
	})
	if runs != 1 {
		t.Fatalf("tool runs = %d, want only the allowed read", runs)
	}
	if text := toolAnswerText(session.Messages, "c2"); text != "tool call refused: fetch is not an allowed tool" {
		t.Fatalf("refusal = %q", text)
	}
}

func TestRegistryRejectsDuplicateNames(t *testing.T) {
	if _, err := NewRegistry(constantTool("read", "a"), constantTool("read", "b")); err == nil {
		t.Fatal("duplicate tool names must be rejected")
	}
}
