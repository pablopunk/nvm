package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// confirmingEcho is the echo tool behind a confirmation.
func confirmingEcho(runs *atomic.Int32) Tool {
	tool := echoTool(runs)
	tool.Name = "delete"
	tool.Confirm = true
	return tool
}

// answering drives a turn: it allows or refuses each approval it sees, and records how many it saw.
func answering(t *testing.T, turn *Turn, allow bool) []Event {
	t.Helper()
	var events []Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == EventApproval && !turn.Decide(event.Call.ID, allow) {
			t.Fatalf("no approval waited for %q", event.Call.ID)
		}
	}
	return events
}

// TestAllowedConfirmationRunsTheCall checks the approval sequence and that the call runs after the allow.
func TestAllowedConfirmationRunsTheCall(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "d", "delete", `{"text":"file"}`)), textTurn("Deleted."))
	turn := Start(context.Background(), setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock()), user("x"))
	events := answering(t, turn, true)
	if got, want := kinds(events), "start step calls tool_start approval tool_end step delta done"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	if runs.Load() != 1 || eventsOf(events, EventToolEnd)[0].Status != CallRan {
		t.Fatalf("runs = %d after an allow", runs.Load())
	}
}

// TestRefusedConfirmationNeverRunsAndTheModelIsTold checks a refusal by the user.
func TestRefusedConfirmationNeverRunsAndTheModelIsTold(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "d", "delete", `{"text":"file"}`)), textTurn("Okay, I won't."))
	turn := Start(context.Background(), setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock()), user("x"))
	events := answering(t, turn, false)
	end := eventsOf(events, EventToolEnd)[0]
	if end.Status != CallRefused || end.Outcome != "Refused by you" || runs.Load() != 0 {
		t.Fatalf("tool_end = %+v, runs = %d", end, runs.Load())
	}
	if terminal(t, events).Kind != EventDone {
		t.Fatalf("the turn did not go on after a refusal: %s", kinds(events))
	}
}

// TestDecideWithoutAWaitingCallReportsFalse checks that a stale or unknown approval is not delivered.
func TestDecideWithoutAWaitingCallReportsFalse(t *testing.T) {
	turn := Start(context.Background(), setup(NewScriptedModel(textTurn("hi")), nil, fake.NewClock()), user("x"))
	if turn.Decide("nobody", true) {
		t.Fatal("Decide reported a waiting call that does not exist")
	}
	drain(t, turn)
}

// TestCancelWhileWaitingForApprovalEndsCancelled checks that Escape during a confirmation runs nothing.
func TestCancelWhileWaitingForApprovalEndsCancelled(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "d", "delete", `{"text":"file"}`)))
	turn := Start(context.Background(), setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock()), user("x"))
	var events []Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == EventApproval {
			turn.Cancel()
		}
	}
	if got, want := kinds(events), "start step calls tool_start approval tool_end aborted"; got != want {
		t.Fatalf("events = %q, want %q", got, want)
	}
	if eventsOf(events, EventToolEnd)[0].Status != CallCancelled || runs.Load() != 0 {
		t.Fatalf("the cancelled approval still ran the tool (%d runs)", runs.Load())
	}
}

// TestAnApprovalIsAnsweredOnce checks that a second decision for the same call finds nothing to wake.
func TestAnApprovalIsAnsweredOnce(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "d", "delete", `{"text":"file"}`)), textTurn("ok"))
	turn := Start(context.Background(), setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock()), user("x"))
	var second bool
	for event := range turn.Events() {
		if event.Kind == EventApproval {
			if !turn.Decide(event.Call.ID, true) {
				t.Fatal("the first decision was not delivered")
			}
			second = turn.Decide(event.Call.ID, true)
		}
	}
	if second {
		t.Fatal("the second decision was delivered")
	}
	_ = json.RawMessage{}
}
