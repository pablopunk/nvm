package aichat

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/search"
)

// awaitingToolSpec is a transcript whose newest tool call waits for the user's answer, and a Send that records each
// message the composer hands it.
func awaitingToolSpec(sent *[]string) ExtensionChatSpec {
	return ExtensionChatSpec{
		Messages: []chat.Message{
			{Author: chat.AuthorUser, Text: "Read my clipboard"},
			{Author: chat.AuthorTool, Status: chat.StatusAwaiting, Tool: &chat.ToolRun{Name: "read_clipboard", Label: "read_clipboard"}},
		},
		Send: func(_ search.Host, text string, _ []chat.Image) { *sent = append(*sent, text) },
	}
}

// TestEmptyEnterSendsAnEmptyMessageWhileAToolCallWaits checks that Enter on an empty composer reaches the view's Send,
// which allows the waiting tool call, instead of being dropped by the composer.
func TestEmptyEnterSendsAnEmptyMessageWhileAToolCallWaits(t *testing.T) {
	var sent []string
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(awaitingToolSpec(&sent))
	if !x.send("") {
		t.Fatal("Enter on an empty composer did not send while a tool call waits")
	}
	if len(sent) != 1 || sent[0] != "" {
		t.Fatalf("sent = %q, want one empty message for the view to allow the call", sent)
	}
}

// TestEmptyEnterSendsNothingWithoutAWaitingToolCall keeps an empty composer inert when no tool call waits.
func TestEmptyEnterSendsNothingWithoutAWaitingToolCall(t *testing.T) {
	var sent []string
	spec := awaitingToolSpec(&sent)
	spec.Messages = spec.Messages[:1]
	x := NewExtensionChat(nil, func() time.Time { return goldenNow })
	x.SetSpec(spec)
	if x.send("") || len(sent) != 0 {
		t.Fatalf("an empty composer sent %q with no tool call waiting", sent)
	}
}
