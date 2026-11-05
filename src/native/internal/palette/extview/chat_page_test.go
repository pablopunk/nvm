package extview

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// chatFixtureView is the dev chat view: a user turn with a picture, a reasoning block still streaming, a waiting tool
// row, and a notice. It is the shape the host draws in the goldens.
func chatFixtureView() api.View {
	return api.View{
		Kind:  api.ViewChat,
		Title: "Dev Chat",
		Chat: &api.ChatSpec{
			Placeholder: "Message AI",
			Tier:        api.ModelSmart,
			Submit:      &api.ActionRef{ID: "send"},
			Stop:        &api.ActionRef{ID: "stop"},
			OnTier:      &api.ActionRef{ID: "tier"},
			Messages: []api.ChatMessage{
				{Role: "system", Content: "Local fixture, no backend."},
				{Role: "user", Content: "What is in this picture?", Images: []api.ChatImage{{Title: "shot.png", DataURL: "data:image/png;base64,AAAA"}}},
				{Role: "assistant", Thinking: "Looking at the image.", Content: "A window.", Streaming: true},
				{Role: "assistant", Tool: &api.ChatTool{Name: "read_file", Status: "waiting", Result: "Allow reading shot.png?"}},
			},
		},
	}
}

// TestChatMessagesKeepTheirPartsAndStatus checks the mapping of each view message to a transcript entry.
func TestChatMessagesKeepTheirPartsAndStatus(t *testing.T) {
	now := time.Unix(100, 0)
	got := chatMessages(chatFixtureView().Chat.Messages, now)
	if len(got) != 4 {
		t.Fatalf("messages = %d, want 4", len(got))
	}
	if got[0].Author != chat.AuthorNotice || got[0].Notice == nil || got[0].Notice.Message == "" {
		t.Fatalf("system line = %+v, want a notice", got[0])
	}
	if got[1].Author != chat.AuthorUser || len(got[1].Images) != 1 || got[1].Images[0].MimeType != "image/png" || got[1].Images[0].Data != "AAAA" {
		t.Fatalf("user turn = %+v, want its picture split into type and data", got[1])
	}
	if got[2].Status != chat.StatusStreaming || got[2].Reasoning != "Looking at the image." || got[2].At != now {
		t.Fatalf("streaming reply = %+v, want streaming status and its reasoning", got[2])
	}
	if got[3].Author != chat.AuthorTool || got[3].Status != chat.StatusAwaiting || got[3].Tool == nil || got[3].Tool.Name != "read_file" {
		t.Fatalf("tool row = %+v, want a waiting tool", got[3])
	}
}

// TestChatTierMapsEmptyToSmart keeps the API's default: an empty tier is smart.
func TestChatTierMapsEmptyToSmart(t *testing.T) {
	if chatTier("") != chat.TierSmart || chatTier(api.ModelFast) != chat.TierFast || chatTier(api.ModelSmart) != chat.TierSmart {
		t.Fatalf("tier mapping is wrong")
	}
}

// TestChatSendRunsSubmitWithTheTypedText checks the composer's text reaches the view's submit handler as the message.
func TestChatSendRunsSubmitWithTheTypedText(t *testing.T) {
	var got api.Invocation
	deps := Deps{Invoke: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) { got = in }}
	s := NewSession(deps, "acme.chat", chatFixtureView())
	s.chatSpec().Send(nil, "hello")
	if got.Ref.ID != "send" || got.Form["message"].Text != "hello" {
		t.Fatalf("submit = %+v, want send with the message", got)
	}
}

// TestChatPrimaryKeyIsStopWhileAReplyStreams checks the footer offers Stop only while a reply streams.
func TestChatPrimaryKeyIsStopWhileAReplyStreams(t *testing.T) {
	s := NewSession(Deps{}, "acme.chat", chatFixtureView())
	page := s.chatPage()
	if page.Primary == nil || page.Primary().Title != "Stop" {
		t.Fatalf("primary while streaming is not Stop")
	}
	done := chatFixtureView()
	for i := range done.Chat.Messages {
		done.Chat.Messages[i].Streaming = false
	}
	s = NewSession(Deps{}, "acme.chat", done)
	if got := s.chatPage().Primary().Title; got != "Send" {
		t.Fatalf("primary after the reply = %q, want Send", got)
	}
}
