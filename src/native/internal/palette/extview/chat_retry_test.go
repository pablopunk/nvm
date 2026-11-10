package extview

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/search"
)

// retryChats is the chat store as the Retry row sees it.
type retryChats struct {
	ChatPages
	calls  []int
	shown  [][]api.ChatMessage
	result string
	err    error
}

func (c *retryChats) Retry(_ string, index int, shown []api.ChatMessage) (string, error) {
	c.calls = append(c.calls, index)
	c.shown = append(c.shown, shown)
	return c.result, c.err
}

func retryView() api.View {
	view := chatFixtureView()
	view.Chat.Messages = []api.ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "answer one"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "answer two"},
	}
	view.Actions = []api.Action{
		{Kind: api.ActionChat, Op: api.ChatRetry, Title: "Retry message 2: answer one", Chat: &api.ChatAction{ChatID: "c1", Index: 1}},
		{Kind: api.ActionChat, Op: api.ChatRetry, Title: "Retry message 4: answer two", Chat: &api.ChatAction{ChatID: "c1", Index: 3}},
	}
	return view
}

// retryPage opens the palette page of a chat and returns it with the chat refs the session opened.
func retryPage(chats *retryChats) (*Session, *[]api.ActionRef) {
	var opened []api.ActionRef
	deps := Deps{Chats: chats, CopyText: func(search.Host, string) {}, Invoke: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) {
		opened = append(opened, in.Ref)
	}}
	return NewSession(deps, aichats.ID, retryView()), &opened
}

func findAction(session *Session, title string) *search.Action {
	for _, action := range session.Palette().Actions() {
		if strings.HasPrefix(action.Title, title) {
			return &action
		}
	}
	return nil
}

func TestRetryRowSitsNextToCopyAndOpensTheNewChat(t *testing.T) {
	chats := &retryChats{result: "c2"}
	s, opened := retryPage(chats)
	titles := map[string]bool{}
	for _, action := range s.Palette().Actions() {
		titles[action.Title] = true
		if strings.HasPrefix(action.Title, "Retry message") && action.Label != "" {
			t.Errorf("%q has the default shortcut %q", action.Title, action.Label)
		}
	}
	if !titles["Retry message 2: answer one"] || !titles["Copy message 2: answer one"] {
		t.Fatalf("menu = %v", titles)
	}
	host := &fakeHost{}
	findAction(s, "Retry message 2").Run(host)
	if !reflect.DeepEqual(chats.calls, []int{1}) || !reflect.DeepEqual(chats.shown[0], retryView().Chat.Messages) {
		t.Fatalf("store got index %v, transcript %+v", chats.calls, chats.shown)
	}
	if len(*opened) != 1 || (*opened)[0].ID != aichats.OpenChatRef("c2").ID || !reflect.DeepEqual((*opened)[0].Args, aichats.OpenChatRef("c2").Args) {
		t.Fatalf("opened %+v, want the new chat", *opened)
	}
	if len(host.toasts) != 0 {
		t.Fatalf("toasts = %q", host.toasts)
	}
}

func TestRetryRefusesAChangedTranscriptAndAReplacedView(t *testing.T) {
	chats := &retryChats{result: "c2"}
	s, opened := retryPage(chats)
	changed := findAction(s, "Retry message 4")
	s.view.Chat.Messages[3].Content = "answer two, longer"
	host := &fakeHost{}
	changed.Run(host)
	replaced := findAction(s, "Retry message 2")
	s.Replace(s.view)
	replaced.Run(host)
	if len(chats.calls) != 0 || len(*opened) != 0 || len(host.toasts) != 2 || host.toasts[0] != chatChangedMessage || host.refresh != 1 {
		t.Fatalf("calls %v, opened %v, toasts %q, refreshes %d", chats.calls, *opened, host.toasts, host.refresh)
	}
}

func TestRetryRefusesAfterDismissalAndShowsStoreRefusals(t *testing.T) {
	chats := &retryChats{err: errors.New("chat: this chat is still answering")}
	s, opened := retryPage(chats)
	action := findAction(s, "Retry message 2")
	host := &fakeHost{}
	action.Run(host)
	if len(host.toasts) != 1 || host.toasts[0] != chats.err.Error() || len(*opened) != 0 {
		t.Fatalf("toasts %q, opened %v", host.toasts, *opened)
	}
	s.dismissed = true
	action.Run(host)
	if len(chats.calls) != 1 || len(host.toasts) != 2 {
		t.Fatalf("dismissed session still retried: calls %v toasts %q", chats.calls, host.toasts)
	}
}

func TestRetryOpensTheNewChatEvenWhenItsFirstReplyCouldNotStart(t *testing.T) {
	chats := &retryChats{result: "c2", err: errors.New("not signed in")}
	s, opened := retryPage(chats)
	host := &fakeHost{}
	findAction(s, "Retry message 4").Run(host)
	if len(host.toasts) != 1 || len(*opened) != 1 {
		t.Fatalf("toasts %q, opened %v", host.toasts, *opened)
	}
}
