package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestDraftingAnnouncesTheToolBeforeItRuns checks the transcript learns what the model is writing as soon as the tool
// name streams, before the call is complete and before it runs.
func TestDraftingAnnouncesTheToolBeforeItRuns(t *testing.T) {
	var runs atomic.Int32
	model := NewScriptedModel(callChunks(call(0, "d", "echo", `{"text":"hi"}`)), textTurn("Done."))
	turn := Start(context.Background(), setup(model, registry(t, echoTool(&runs)), fake.NewClock()), user("x"))
	events := drain(t, turn)
	if len(events) < 3 || events[2].Kind != EventCallDrafting || events[3].Kind != EventCalls {
		t.Fatalf("events = %s, want call_drafting right before calls", kinds(events))
	}
	if drafts := eventsOf(events, EventCallDrafting); len(drafts) != 1 || drafts[0].Call.Label != "Echoing" {
		t.Fatalf("drafting events = %+v, want one for the echo tool's label", drafts)
	}
}
