package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestExtensionChatMessagesCarryRolesAndToolRows checks the transcript an extension reads: a user turn keeps its
// pictures, a reply keeps its reasoning and streaming state, and a tool row keeps its status word and result.
func TestExtensionChatMessagesCarryRolesAndToolRows(t *testing.T) {
	user := apiChatMessage(chat.Message{Author: chat.AuthorUser, Text: "look", Images: []chat.Image{{Name: "a.png", MimeType: "image/png", Data: "AAAA"}}})
	if user.Role != "user" || len(user.Images) != 1 || user.Images[0].DataURL != "data:image/png;base64,AAAA" {
		t.Fatalf("user turn = %+v, want its picture as a data URL", user)
	}
	reply := apiChatMessage(chat.Message{Author: chat.AuthorAssistant, Text: "a window", Reasoning: "checking", Status: chat.StatusStreaming})
	if reply.Role != "assistant" || reply.Thinking != "checking" || !reply.Streaming {
		t.Fatalf("reply = %+v, want reasoning and streaming", reply)
	}
	tool := apiChatMessage(chat.Message{Author: chat.AuthorTool, Status: chat.StatusAwaiting, Tool: &chat.ToolRun{Name: "read_file", Outcome: "Allow?"}})
	if tool.Tool == nil || tool.Tool.Status != "waiting" || tool.Tool.Name != "read_file" || tool.Tool.Result != "Allow?" {
		t.Fatalf("tool row = %+v, want a waiting read_file", tool.Tool)
	}
	if got := apiChatMessage(chat.Message{Author: chat.AuthorNotice, Notice: &chat.Notice{Message: "Credits are low."}}); got.Role != "system" || got.Content != "Credits are low." {
		t.Fatalf("notice = %+v, want a system line", got)
	}
}

// TestChatActionsNeedTheirPayload checks an ActionChat without its payload is refused by the API, as the host would
// refuse it.
func TestChatActionsNeedTheirPayload(t *testing.T) {
	if err := (api.Action{Kind: api.ActionChat, Op: api.ChatOpen}).Validate(); err == nil {
		t.Fatalf("a chat action without its payload validated")
	}
	if err := (api.Action{Kind: api.ActionChat, Op: api.ChatOpen, Chat: &api.ChatAction{ChatID: "c1"}}).Validate(); err != nil {
		t.Fatalf("a chat action with its payload: %v", err)
	}
}
