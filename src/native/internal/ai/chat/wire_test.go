package chat

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// TestReasoningIsKeptApartFromReplyText checks reasoning is its own chunk: it never adds to the reply text, so a turn
// that only reasons keeps its reply empty until visible text arrives.
func TestReasoningIsKeptApartFromReplyText(t *testing.T) {
	chunk, ok, err := chunkOf(backend.Event{Type: backend.EventReasoning, Text: "weighing the options"})
	if !ok || err != nil || chunk.Text != "" || chunk.Reasoning != "weighing the options" {
		t.Fatalf("chunkOf(reasoning) = %+v ok=%v err=%v, want the reasoning apart from the text", chunk, ok, err)
	}
}
