package chat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TestReasoningEventsCarryNoReplyText checks reasoning is not reply text: a turn that only reasons has no chunk, so its
// reply keeps the thinking state until visible text arrives.
func TestReasoningEventsCarryNoReplyText(t *testing.T) {
	chunk, ok, err := chunkOf(backend.Event{Type: backend.EventReasoning, Text: "weighing the options"})
	if ok || err != nil || chunk.Text != "" {
		t.Fatalf("chunkOf(reasoning) = %+v ok=%v err=%v, want no chunk", chunk, ok, err)
	}
}
