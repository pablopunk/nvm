package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// stepTool is a tool that does one thing for the user and asks before it: the shape of a program's destructive call.
func stepTool(effects *atomic.Int32, asked *atomic.Int32) Tool {
	return Tool{
		Name: "act", Label: "Acting", Schema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
		Timeout: -1,
		Run: func(ctx context.Context, _ json.RawMessage) (Result, error) {
			steps := SubCallsFrom(ctx)
			step := steps.Begin("Deleting a note", "Delete note “Old”")
			asked.Add(1)
			allowed, err := step.Ask(ctx)
			if err != nil {
				step.End(CallCancelled, "Stopped before it ran", "")
				return Result{}, err
			}
			if !allowed {
				step.End(CallRefused, "Refused by you", "")
				return Result{Text: "declined", Failed: true, Outcome: "Declined"}, nil
			}
			effects.Add(1)
			step.End(CallRan, "Deleted", "deleted")
			return Result{Text: "deleted", Outcome: "Done"}, nil
		},
	}
}

func TestSubCallRowsNestUnderTheirCallAndWaitForTheUser(t *testing.T) {
	var effects, asked atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "p1", "act", `{}`)), textTurn("Done."))
	turn := Start(context.Background(), setup(model, registry(t, stepTool(&effects, &asked)), fake.NewClock()), user("x"))
	events := answering(t, turn, true)
	if got, want := kinds(events), "start step calls tool_start tool_start approval tool_end tool_end step delta done"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	nested := eventsOf(events, EventToolStart)[1].Call
	if nested.ParentID != "p1" || nested.ID != "p1:1" || nested.Label != "Deleting a note" || nested.Input != "Delete note “Old”" {
		t.Fatalf("nested call = %+v", nested)
	}
	if approval := eventsOf(events, EventApproval)[0].Call; approval.ID != "p1:1" {
		t.Fatalf("the approval is for %q; want the nested call", approval.ID)
	}
	if effects.Load() != 1 {
		t.Fatalf("effects = %d after an allow", effects.Load())
	}
	if end := eventsOf(events, EventToolEnd)[1]; end.Call.ID != "p1" || end.Status != CallRan {
		t.Fatalf("the call ended as %+v", end)
	}
}

func TestDeclinedSubCallDoesNotRunAndTheCallEndsFailed(t *testing.T) {
	var effects, asked atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "p1", "act", `{}`)), textTurn("Okay."))
	turn := Start(context.Background(), setup(model, registry(t, stepTool(&effects, &asked)), fake.NewClock()), user("x"))
	events := answering(t, turn, false)
	ends := eventsOf(events, EventToolEnd)
	if ends[0].Status != CallRefused || ends[0].Outcome != "Refused by you" || ends[1].Status != CallFailed {
		t.Fatalf("ends = %+v", ends)
	}
	if effects.Load() != 0 {
		t.Fatal("a declined step ran")
	}
}

// Stopping the turn while a step waits for the user ends the step and the call as cancelled, and nothing runs.
func TestStopWhileASubCallWaitsCancelsTheCallAndRunsNothing(t *testing.T) {
	var effects, asked atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "p1", "act", `{}`)))
	turn := Start(context.Background(), setup(model, registry(t, stepTool(&effects, &asked)), fake.NewClock()), user("x"))
	var events []Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == EventApproval {
			turn.Cancel()
		}
	}
	ends := eventsOf(events, EventToolEnd)
	if len(ends) != 2 || ends[0].Status != CallCancelled || ends[1].Status != CallCancelled {
		t.Fatalf("ends = %+v", ends)
	}
	if effects.Load() != 0 || terminal(t, events).Kind != EventAborted {
		t.Fatalf("effects = %d, terminal = %s", effects.Load(), kinds(events))
	}
	if turn.Decide("p1:1", true) {
		t.Fatal("the cancelled approval still waits")
	}
}

// The user's time answering a step does not count against the tool's wall clock or the turn's active time.
func TestASubCallWaitDoesNotCountAgainstTheTurnDeadline(t *testing.T) {
	var effects, asked atomic.Int32
	clock := fake.NewClock()
	model := NewScriptedModel(callChunks(call(0, "p1", "act", `{}`)), textTurn("Done."))
	s := setup(model, registry(t, stepTool(&effects, &asked)), clock)
	s.TurnTimeout = time.Minute
	turn := Start(context.Background(), s, user("x"))
	var events []Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == EventApproval {
			clock.Advance(10 * time.Minute)
			turn.Decide(event.Call.ID, true)
		}
	}
	if terminal(t, events).Kind != EventDone || effects.Load() != 1 {
		t.Fatalf("the turn ended %s after a long wait; effects %d", kinds(events), effects.Load())
	}
}

func TestToolWithoutATurnHasNoSubCalls(t *testing.T) {
	if SubCallsFrom(context.Background()) != nil {
		t.Fatal("a bare context carries sub calls")
	}
}
