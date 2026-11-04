package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestApprovalWaitDoesNotCountTowardTheTurnLimit checks the turn limit measures active time: a user who takes longer
// than the limit to allow a tool call does not end the turn, and the call still runs.
func TestApprovalWaitDoesNotCountTowardTheTurnLimit(t *testing.T) {
	var runs atomic.Int32
	clock := fake.NewClock()
	model := NewScriptedModel(callChunks(call(0, "d", "delete", `{"text":"file"}`)), textTurn("Deleted."))
	limited := setup(model, registry(t, confirmingEcho(&runs)), clock)
	limited.TurnTimeout = time.Minute
	turn := Start(context.Background(), limited, user("x"))

	var events []Event
	for event := range turn.Events() {
		events = append(events, event)
		if event.Kind == EventApproval {
			clock.Advance(3 * time.Minute)
			if !turn.Decide(event.Call.ID, true) {
				t.Fatalf("no approval waited for %q", event.Call.ID)
			}
		}
	}
	if terminal(t, events).Kind != EventDone || runs.Load() != 1 {
		t.Fatalf("events = %s, runs = %d; want the turn to finish and the call to run", kinds(events), runs.Load())
	}
}

// TestActiveTimePastTheLimitEndsTheTurnWithTheDeadline checks a turn whose active time runs past the limit ends as a
// deadline error, not as a stop: here a tool run takes the time, and the next model step is refused.
func TestActiveTimePastTheLimitEndsTheTurnWithTheDeadline(t *testing.T) {
	clock := fake.NewClock()
	slow := Tool{Name: "slow", Label: "Slow", Schema: echoSchema, Run: func(context.Context, json.RawMessage) (Result, error) {
		clock.Advance(2 * time.Minute)
		return Result{Text: "done", Outcome: "Done"}, nil
	}}
	model := NewScriptedModel(callChunks(call(0, "s", "slow", `{"text":"x"}`)), textTurn("never reached"))
	limited := setup(model, registry(t, slow), clock)
	limited.TurnTimeout = time.Minute
	turn := Start(context.Background(), limited, user("x"))
	events := drain(t, turn)
	if end := terminal(t, events); end.Kind != EventError || end.Err != ErrTurnDeadline {
		t.Fatalf("terminal = %+v, want the deadline error", end)
	}
}
