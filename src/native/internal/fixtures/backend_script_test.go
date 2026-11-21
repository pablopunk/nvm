package fixtures

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestScriptedRepliesAnswerInOrderThenTheReplyText checks the queue the rig uses to script a model that calls a tool
// and then answers with its result: each chat request takes the next queued reply, and the reply text returns once the
// queue is empty.
func TestScriptedRepliesAnswerInOrderThenTheReplyText(t *testing.T) {
	backend := NewBackend()
	backend.SetScript(
		ToolCallReply("", ToolCall{ID: "call_1", Name: "read_clipboard", Arguments: []string{"{}"}}),
		TextReply("after the tool"),
	)
	set := fake.NewSet()
	set.HTTP.Handle(backend)

	_, _, first := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if !strings.Contains(first, `"name":"read_clipboard"`) || strings.Contains(first, "after the tool") {
		t.Fatalf("first reply = %q, want the tool call only", first)
	}
	_, _, second := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if got := assembleStream(t, second); got != "after the tool" {
		t.Fatalf("second reply %q, want the scripted text", got)
	}
	_, _, third := doThroughPort(t, set, context.Background(), http.MethodPost, ChatCompletionsPath, chatRequestBody)
	if got := assembleStream(t, third); got != FixtureReply {
		t.Fatalf("reply after the script %q, want the reply text %q", got, FixtureReply)
	}
}
