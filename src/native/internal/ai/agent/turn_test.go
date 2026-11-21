package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestTextOnlyTurnStreamsDeltasThenDone is the simplest golden sequence: start, one model turn, done with usage.
func TestTextOnlyTurnStreamsDeltasThenDone(t *testing.T) {
	model := NewScriptedModel(textTurn("Hel", "lo"))
	turn := Start(context.Background(), setup(model, nil, fake.NewClock()), user("hi"))
	events := drain(t, turn)
	if got, want := kinds(events), "start step delta delta done"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	if got := events[2].Text + events[3].Text; got != "Hello" {
		t.Fatalf("streamed text = %q", got)
	}
	if model.Closes() != 1 {
		t.Fatalf("streams closed = %d, want 1", model.Closes())
	}
}

// TestToolCallRunsAndTheModelReadsItsResult covers a call whose arguments arrive in pieces: the loop joins them,
// runs the tool, sends its result back and ends on the model's answer.
func TestToolCallRunsAndTheModelReadsItsResult(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(
		callChunks(call(0, "call_a", "echo", `{"te`), call(0, "", "", `xt":"ping"}`)),
		textTurn("Done."),
	)
	turn := Start(context.Background(), setup(model, registry(t, echoTool(&runs)), fake.NewClock()), user("echo ping"))
	events := drain(t, turn)
	want := "start step calls tool_start tool_end step delta done"
	if got := kinds(events); got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	if runs.Load() != 1 {
		t.Fatalf("tool runs = %d, want 1", runs.Load())
	}
	end := eventsOf(events, EventToolEnd)[0]
	if end.Status != CallRan || end.Result != "echo: ping" || end.Outcome != "Echoed" || end.Call.Input != "ping" {
		t.Fatalf("tool_end = %+v", end)
	}
	second := model.Requests()[1].Messages
	last := second[len(second)-1]
	if last.Role != backend.RoleTool || last.ToolCallID != "call_a" || string(last.Content) != `"echo: ping"` {
		t.Fatalf("second request ends with %+v, want the tool result", last)
	}
	if model.Requests()[0].Tools == nil {
		t.Fatal("the first request offered no tools")
	}
}

// TestParallelCallsRunInOrderAndEachGetsOneAnswer checks two calls in one model turn.
func TestParallelCallsRunInOrderAndEachGetsOneAnswer(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(
		callChunks(call(0, "a", "echo", `{"text":"one"}`), call(1, "b", "echo", `{"text":"two"}`)),
		textTurn("Both."),
	)
	turn := Start(context.Background(), setup(model, registry(t, echoTool(&runs)), fake.NewClock()), user("x"))
	events := drain(t, turn)
	if got, want := kinds(events), "start step calls tool_start tool_end tool_start tool_end step delta done"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	answers := 0
	for _, message := range model.Requests()[1].Messages {
		if message.Role == backend.RoleTool {
			answers++
		}
	}
	if answers != 2 || runs.Load() != 2 {
		t.Fatalf("answers = %d, runs = %d; want 2 and 2", answers, runs.Load())
	}
}

// TestUnknownToolIsAnsweredAndNeverRuns checks a name the registry does not hold.
func TestUnknownToolIsAnsweredAndNeverRuns(t *testing.T) {
	model := NewScriptedModel(callChunks(call(0, "u", "format_disk", `{}`)), textTurn("Sorry."))
	turn := Start(context.Background(), setup(model, registry(t), fake.NewClock()), user("x"))
	events := drain(t, turn)
	end := eventsOf(events, EventToolEnd)[0]
	if end.Status != CallFailed || end.Outcome != "Unknown tool" || !strings.Contains(end.Result, "not a registered tool") {
		t.Fatalf("tool_end = %+v", end)
	}
	if terminal(t, events).Kind != EventDone {
		t.Fatalf("the turn did not finish after the unknown call: %s", kinds(events))
	}
}

// TestMalformedArgumentsAreRefusedBeforeTheTool is the table of argument failures: each one stops before the tool.
func TestMalformedArgumentsAreRefusedBeforeTheTool(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
	}{
		{"broken JSON", `{"text":`},
		{"not an object", `["ping"]`},
		{"missing required field", `{}`},
		{"wrong type", `{"text":3}`},
		{"unknown field", `{"text":"x","extra":1}`},
		{"too long", `{"text":"` + strings.Repeat("a", 201) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runs atomic.Int32
			model := NewScriptedModel(callChunks(call(0, "m", "echo", tt.arguments)), textTurn("ok"))
			turn := Start(context.Background(), setup(model, registry(t, echoTool(&runs)), fake.NewClock()), user("x"))
			events := drain(t, turn)
			end := eventsOf(events, EventToolEnd)[0]
			if end.Status != CallFailed || end.Outcome != "Invalid arguments" {
				t.Fatalf("tool_end = %+v", end)
			}
			if runs.Load() != 0 {
				t.Fatal("the tool ran with bad arguments")
			}
		})
	}
}

// TestPolicyRefusalNeverRunsTheTool checks that a refusal reaches the model as a refused call, not a failure.
func TestPolicyRefusalNeverRunsTheTool(t *testing.T) {
	var runs atomic.Int32
	policy := refusingPolicy{}
	model := NewScriptedModel(callChunks(call(0, "r", "echo", `{"text":"x"}`)), textTurn("ok"))
	turn := Start(context.Background(), Setup{Model: model, Tools: registry(t, echoTool(&runs)), Policy: policy, Clock: fake.NewClock()}, user("x"))
	events := drain(t, turn)
	end := eventsOf(events, EventToolEnd)[0]
	if end.Status != CallRefused || end.Outcome != "Refused: outside the allowed folder" || runs.Load() != 0 {
		t.Fatalf("tool_end = %+v, runs = %d", end, runs.Load())
	}
}

// refusingPolicy refuses every call with one reason.
type refusingPolicy struct{}

func (refusingPolicy) Decide(backend.ToolCall, Tool) Decision {
	return Refuse("outside the allowed folder")
}

// TestStepLimitEndsARunawayTurn scripts a model that asks for a tool on every turn.
func TestStepLimitEndsARunawayTurn(t *testing.T) {
	var runs atomic.Int32
	var turns []ScriptedTurn
	for i := 0; i < 3; i++ {
		turns = append(turns, callChunks(call(0, "loop", "echo", `{"text":"again"}`)))
	}
	model := NewScriptedModel(turns...)
	s := setup(model, registry(t, echoTool(&runs)), fake.NewClock())
	s.MaxSteps = 2
	events := drain(t, Start(context.Background(), s, user("loop")))
	last := terminal(t, events)
	if last.Kind != EventError || !errors.Is(last.Err, ErrStepLimit) {
		t.Fatalf("terminal = %+v, want the step limit", last)
	}
	if runs.Load() != 2 || len(model.Requests()) != 2 {
		t.Fatalf("runs = %d, model turns = %d; want 2 and 2", runs.Load(), len(model.Requests()))
	}
}

// TestModelFailureKeepsItsTypeAndTheTextBeforeIt checks that a failed reply reaches the caller as the same error, after
// the text it had streamed.
func TestModelFailureKeepsItsTypeAndTheTextBeforeIt(t *testing.T) {
	failure := &backend.Error{Kind: backend.KindRateLimited, Status: 429}
	model := NewScriptedModel(ScriptedTurn{Chunks: []ModelChunk{{Text: "Part"}}, Err: failure})
	events := drain(t, Start(context.Background(), setup(model, nil, fake.NewClock()), user("x")))
	if got := kinds(events); got != "start step delta error" {
		t.Fatalf("events = %q", got)
	}
	var typed *backend.Error
	if !errors.As(terminal(t, events).Err, &typed) || typed != failure {
		t.Fatalf("error = %v, want the backend error itself", terminal(t, events).Err)
	}
}

// TestUsageIsSummedAcrossTurns checks that done reports the tokens of every model turn.
func TestUsageIsSummedAcrossTurns(t *testing.T) {
	var runs atomic.Int32
	usage := func(in, out int) ModelChunk { return ModelChunk{Usage: &Usage{InputTokens: in, OutputTokens: out}} }
	first := callChunks(call(0, "u1", "echo", `{"text":"a"}`))
	first.Chunks = append(first.Chunks, usage(10, 2))
	second := textTurn("ok")
	second.Chunks = append(second.Chunks, usage(20, 3))
	events := drain(t, Start(context.Background(), setup(NewScriptedModel(first, second), registry(t, echoTool(&runs)), fake.NewClock()), user("x")))
	if got := terminal(t, events).Usage; got != (Usage{InputTokens: 30, OutputTokens: 5}) {
		t.Fatalf("usage = %+v", got)
	}
}

// TestCancelDuringAToolAnswersEveryCall starts two calls, cancels while the first runs, and checks that both calls end
// as cancelled and the turn ends as aborted.
func TestCancelDuringAToolAnswersEveryCall(t *testing.T) {
	started := make(chan struct{})
	blocking := Tool{
		Name: "wait", Label: "Waiting", Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			close(started)
			<-ctx.Done()
			return Result{}, ctx.Err()
		},
	}
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "w1", "wait", `{}`), call(1, "w2", "echo", `{"text":"x"}`)))
	turn := Start(context.Background(), setup(model, registry(t, blocking, echoTool(&runs)), fake.NewClock()), user("x"))
	var events []Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == EventToolStart && event.Call.ID == "w1" {
			<-started
			turn.Cancel()
		}
	}
	if got, want := kinds(events), "start step calls tool_start tool_end tool_start tool_end aborted"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	ends := eventsOf(events, EventToolEnd)
	if ends[0].Status != CallCancelled || ends[1].Status != CallCancelled || runs.Load() != 0 {
		t.Fatalf("tool ends = %+v; the second tool ran %d times", ends, runs.Load())
	}
	if ends[1].Result != abandonedResult {
		t.Fatalf("the unrun call reads %q", ends[1].Result)
	}
}

// TestCancelBeforeTheModelAnswersEndsAborted checks a turn cancelled while the model is still waiting.
func TestCancelBeforeTheModelAnswersEndsAborted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events := drain(t, Start(ctx, setup(NewScriptedModel(textTurn("never")), nil, fake.NewClock()), user("x")))
	if got := kinds(events); got != "start aborted" {
		t.Fatalf("events = %q", got)
	}
}

// TestPanickingToolFailsWithoutCrashingTheLoop checks that a tool panic is a failed call.
func TestPanickingToolFailsWithoutCrashingTheLoop(t *testing.T) {
	boom := Tool{Name: "boom", Label: "Booming", Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(context.Context, json.RawMessage) (Result, error) { panic("out of cheese") }}
	model := NewScriptedModel(callChunks(call(0, "p", "boom", `{}`)), textTurn("recovered"))
	events := drain(t, Start(context.Background(), setup(model, registry(t, boom), fake.NewClock()), user("x")))
	end := eventsOf(events, EventToolEnd)[0]
	if end.Status != CallFailed || end.Outcome != "Failed unexpectedly" || terminal(t, events).Kind != EventDone {
		t.Fatalf("tool_end = %+v; events %s", end, kinds(events))
	}
}

// TestToolOutputIsCappedForTheModel checks the output cap, which keeps transcripts bounded.
func TestToolOutputIsCappedForTheModel(t *testing.T) {
	big := Tool{Name: "big", Label: "Big", Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(context.Context, json.RawMessage) (Result, error) {
			return Result{Text: strings.Repeat("é", 2000)}, nil
		}}
	model := NewScriptedModel(callChunks(call(0, "b", "big", `{}`)), textTurn("ok"))
	s := setup(model, registry(t, big), fake.NewClock())
	s.MaxOutputBytes = 1001
	events := drain(t, Start(context.Background(), s, user("x")))
	result := eventsOf(events, EventToolEnd)[0].Result
	if !strings.Contains(result, "[output truncated: kept 1000 of 4000 bytes]") || len(result) > 1100 {
		t.Fatalf("result = %q", result[len(result)-80:])
	}
}

// TestToolTimeoutIsMeasuredOnTheClock checks that a tool which outlives its deadline is reported as timed out, using
// the fake clock so no test waits on the wall clock.
func TestToolTimeoutIsMeasuredOnTheClock(t *testing.T) {
	stuck := Tool{Name: "stuck", Label: "Stuck", Schema: json.RawMessage(`{"type":"object"}`),
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}}
	clock := fake.NewClock()
	model := NewScriptedModel(callChunks(call(0, "s", "stuck", `{}`)), textTurn("gave up"))
	s := setup(model, registry(t, stuck), clock)
	s.ToolTimeout = 5 * time.Second
	turn := Start(context.Background(), s, user("x"))
	drained := make(chan []Event, 1)
	go func() { drained <- drain(t, turn) }()
	clock.AwaitTimerDueIn(s.ToolTimeout)
	clock.Advance(s.ToolTimeout)
	events := <-drained
	end := eventsOf(events, EventToolEnd)[0]
	if end.Status != CallFailed || end.Outcome != "Timed out" {
		t.Fatalf("tool_end = %+v", end)
	}
}

// TestEachStreamIsClosedOnce checks the close count with a tool loop: one stream per model turn.
func TestEachStreamIsClosedOnce(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "c", "echo", `{"text":"x"}`)), textTurn("ok"))
	drain(t, Start(context.Background(), setup(model, registry(t, echoTool(&runs)), fake.NewClock()), user("x")))
	if model.Closes() != 2 {
		t.Fatalf("closes = %d, want 2", model.Closes())
	}
}
