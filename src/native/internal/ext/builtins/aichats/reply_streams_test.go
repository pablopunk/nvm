package aichats

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// A program's nested step that ended last must not stop the transcript's refresh while the program still runs above it:
// the refused step of a run_program call left the chat stuck on "Running a program".
func TestTheTranscriptKeepsRefreshingWhileAProgramRunsAboveAnEndedStep(t *testing.T) {
	messages := []api.ChatMessage{
		{Role: "user", Content: "Bind clipboard history to a shortcut"},
		{Role: "assistant", Tool: &api.ChatTool{Name: "run_program", Status: "running"}},
		{Role: "assistant", Tool: &api.ChatTool{Name: "run_program.step", Status: "refused"}},
	}
	if !replyStreams(messages) {
		t.Fatal("the transcript stopped refreshing while the program still ran")
	}
	if view := chatView(nil, api.ChatDetail{ChatSummary: api.ChatSummary{ID: "chat"}, Messages: messages}); view.Refresh == nil {
		t.Fatal("the chat view has no refresh while the program still runs")
	}
}

// An earlier turn's tool row does not keep a finished chat refreshing.
func TestAFinishedChatStopsRefreshing(t *testing.T) {
	messages := []api.ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Tool: &api.ChatTool{Name: "run_program", Status: "running"}},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "done"},
	}
	if replyStreams(messages) {
		t.Fatal("a finished reply still refreshes")
	}
}
