package spike

import (
	"context"
	"reflect"
	"testing"
)

// The six golden scenarios. Each asserts the exact event sequence; the mock model makes every run deterministic.

func TestGoldenPlainTextAnswer(t *testing.T) {
	model := NewMockModel(MockTurn{Chunks: textChunks("Hel", "lo")})
	agent := NewAgent(model, newTestRegistry(t), Options{})

	got := summarize(drain(agent.Run(context.Background(), &Session{}, UserMessage{Text: "hi"})))

	assertSequence(t, got, []string{"start", "delta:Hel", "delta:lo", "done"})
}

func TestGoldenSingleToolCallThenAnswer(t *testing.T) {
	model := NewMockModel(
		MockTurn{Chunks: callChunks(0, "c1", "read", `{"path":`, `"notes.md"}`)},
		MockTurn{Chunks: textChunks("It says hi")},
	)
	agent := NewAgent(model, newTestRegistry(t, constantTool("read", "hi")), Options{})

	got := summarize(drain(agent.Run(context.Background(), &Session{}, UserMessage{Text: "read it"})))

	assertSequence(t, got, []string{"start", "tool_start:read", "tool_end:read:ok", "delta:It says hi", "done"})
}

func TestGoldenParallelToolCallsRunInCallOrder(t *testing.T) {
	model := NewMockModel(
		MockTurn{Chunks: concatChunks(
			callChunks(0, "c1", "read", `{"path":"a.md"}`),
			callChunks(1, "c2", "grep", `{"pattern":"x"}`),
		)},
		MockTurn{Chunks: textChunks("Both done")},
	)
	agent := NewAgent(model, newTestRegistry(t, constantTool("read", "A"), constantTool("grep", "B")), Options{})

	session := &Session{}
	got := summarize(drain(agent.Run(context.Background(), session, UserMessage{Text: "both"})))

	assertSequence(t, got, []string{
		"start",
		"tool_start:read", "tool_end:read:ok",
		"tool_start:grep", "tool_end:grep:ok",
		"delta:Both done", "done",
	})
	second := model.Requests()[1].Messages
	if answered := toolAnswerIDs(second); !reflect.DeepEqual(answered, []string{"c1", "c2"}) {
		t.Fatalf("tool answers in request 2 = %v, want [c1 c2]", answered)
	}
}

func TestGoldenMalformedArgumentsAreReportedNotRun(t *testing.T) {
	runs := 0
	model := NewMockModel(
		MockTurn{Chunks: callChunks(0, "c1", "read", `{"path":`)},
		MockTurn{Chunks: textChunks("Retrying without arguments")},
	)
	agent := NewAgent(model, newTestRegistry(t, countingTool("read", "never", &runs)), Options{})

	session := &Session{}
	got := summarize(drain(agent.Run(context.Background(), session, UserMessage{Text: "read"})))

	assertSequence(t, got, []string{"start", "tool_start:read", "tool_end:read:error", "delta:Retrying without arguments", "done"})
	if runs != 0 {
		t.Fatalf("tool ran %d times with malformed arguments, want 0", runs)
	}
	if text := toolAnswerText(session.Messages, "c1"); text != "tool arguments must be a JSON object" {
		t.Fatalf("tool answer = %q", text)
	}
}

func TestGoldenToolFailureGoesBackToTheModel(t *testing.T) {
	model := NewMockModel(
		MockTurn{Chunks: callChunks(0, "c1", "fetch", `{"url":"https://example.invalid"}`)},
		MockTurn{Chunks: textChunks("The fetch failed.")},
	)
	agent := NewAgent(model, newTestRegistry(t, failingTool("fetch", errConnectionRefused)), Options{})

	session := &Session{}
	got := summarize(drain(agent.Run(context.Background(), session, UserMessage{Text: "fetch"})))

	assertSequence(t, got, []string{"start", "tool_start:fetch", "tool_end:fetch:error", "delta:The fetch failed.", "done"})
	if text := toolAnswerText(session.Messages, "c1"); text != "tool failed: connection refused" {
		t.Fatalf("tool answer = %q", text)
	}
}

func TestGoldenEndlessToolCallsHitTheTurnCap(t *testing.T) {
	model := NewRepeatingMockModel(MockTurn{Chunks: callChunks(0, "loop", "ls", `{}`)})
	runs := 0
	agent := NewAgent(model, newTestRegistry(t, countingTool("ls", "entries", &runs)), Options{MaxModelTurns: 3})

	got := summarize(drain(agent.Run(context.Background(), &Session{}, UserMessage{Text: "list forever"})))

	want := []string{
		"start",
		"tool_start:ls", "tool_end:ls:ok",
		"tool_start:ls", "tool_end:ls:ok",
		"tool_start:ls", "tool_end:ls:ok",
		"error:iteration_limit",
	}
	assertSequence(t, got, want)
	if n := len(model.Requests()); n != 3 {
		t.Fatalf("model turns = %d, want the cap of 3", n)
	}
	if runs != 3 {
		t.Fatalf("tool runs = %d, want 3", runs)
	}
}

func assertSequence(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event sequence\n got: %q\nwant: %q", got, want)
	}
}

func toolAnswerIDs(messages []Message) []string {
	var ids []string
	for _, message := range messages {
		if message.Role == RoleTool {
			ids = append(ids, message.ToolCallID)
		}
	}
	return ids
}

func toolAnswerText(messages []Message, callID string) string {
	for _, message := range messages {
		if message.Role == RoleTool && message.ToolCallID == callID {
			return message.Text
		}
	}
	return ""
}
