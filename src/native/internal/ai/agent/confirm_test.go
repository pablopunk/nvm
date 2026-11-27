package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// A call the user refused is not asked again in the turn, even spelled another way: it fails with a clear reason.
func TestARetryAfterARefusalIsNotAskedAgain(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(
		callChunks(call(0, "d1", "delete", `{"text":"file"}`)),
		callChunks(call(0, "d2", "delete", `{ "text" : "file" }`)),
		textTurn("Okay."))
	turn := Start(context.Background(), setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock()), user("x"))
	events := answering(t, turn, false)
	if approvals := len(eventsOf(events, EventApproval)); approvals != 1 || runs.Load() != 0 {
		t.Fatalf("asked %d times and ran %d; want one question and no run", approvals, runs.Load())
	}
	retry := eventsOf(events, EventToolEnd)[1]
	if retry.Status != CallRefused || retry.Outcome != "Refused by you earlier" || !strings.Contains(retry.Result, "already refused this exact tool call") {
		t.Fatalf("the retry ended %+v", retry)
	}
}

// A refusal covers only its exact call: the same tool on another resource is asked.
func TestARefusalDoesNotCoverAnotherResource(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(
		callChunks(call(0, "d1", "delete", `{"text":"a"}`)),
		callChunks(call(0, "d2", "delete", `{"text":"b"}`)),
		textTurn("Okay."))
	turn := Start(context.Background(), setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock()), user("x"))
	if approvals := len(eventsOf(answering(t, turn, false), EventApproval)); approvals != 2 {
		t.Fatalf("asked %d times; want both resources asked", approvals)
	}
}

// onceWaiver waives the first call whose arguments it names.
type onceWaiver struct {
	arguments string
	spent     atomic.Bool
}

func (w *onceWaiver) Waive(call backend.ToolCall, _ Tool) bool {
	return CanonicalArguments(json.RawMessage(call.Function.Arguments)) == w.arguments && w.spent.CompareAndSwap(false, true)
}

// A waived call runs without its question; the host's waiver decides, and the next call asks again.
func TestAWaivedCallRunsWithoutAskingAndTheNextOneAsks(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(
		callChunks(call(0, "d1", "delete", `{"text":"file"}`)),
		callChunks(call(0, "d2", "delete", `{"text":"file"}`)),
		textTurn("Done."))
	s := setup(model, registry(t, confirmingEcho(&runs)), fake.NewClock())
	s.Waiver = &onceWaiver{arguments: `{"text":"file"}`}
	events := answering(t, Start(context.Background(), s, user("x")), true)
	if approvals := len(eventsOf(events, EventApproval)); approvals != 1 || runs.Load() != 2 {
		t.Fatalf("asked %d times and ran %d; want the second call asked and both run", approvals, runs.Load())
	}
	if first := eventsOf(events, EventApproval)[0]; first.Call.ID != "d2" {
		t.Fatalf("asked about %q; want only the call after the waived one", first.Call.ID)
	}
}

func TestCanonicalArgumentsNameOneResourceForEverySpelling(t *testing.T) {
	if CanonicalArguments(json.RawMessage(`{"b":1,"a":"x"}`)) != CanonicalArguments(json.RawMessage(` { "a" : "x", "b" : 1 } `)) {
		t.Fatal("two spellings of one call named two resources")
	}
	if CanonicalArguments(nil) != CanonicalArguments(json.RawMessage("null")) {
		t.Fatal("no arguments and null named two resources")
	}
	if CanonicalArguments(json.RawMessage(`{"a":1}`)) == CanonicalArguments(json.RawMessage(`{"a":2}`)) {
		t.Fatal("two resources were named as one")
	}
}
