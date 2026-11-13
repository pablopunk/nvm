package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// recordingChats is the chat store as the continue action sees it: it records each message Send receives.
type recordingChats struct {
	ChatPages
	sent []string
}

func (c *recordingChats) Send(_, text string, _ []chat.Image) error {
	c.sent = append(c.sent, text)
	return nil
}

// TestEmptyContinueReachesTheChatStore checks that an empty composer's Enter is passed to the store, which allows a tool
// call that waits. The action must not drop it, or the approval row never clears.
func TestEmptyContinueReachesTheChatStore(t *testing.T) {
	chats := &recordingChats{}
	s := NewSession(Deps{Chats: chats, Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) {}}, "acme.chat", chatFixtureView())
	action := s.chatOperation(api.Action{Kind: api.ActionChat, Op: api.ChatContinue, Chat: &api.ChatAction{ChatID: "c1"}}, "Continue Chat")
	host := &fakeHost{}
	action.Run(host)
	if len(chats.sent) != 1 || chats.sent[0] != "" {
		t.Fatalf("store received %q, want one empty message", chats.sent)
	}
	if len(host.toasts) != 0 {
		t.Fatalf("toasts = %q, want none for an empty continue", host.toasts)
	}
}
