package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

// openedChatPage opens a seeded chat through the built-in and returns its palette page, with the chats the page asked
// the built-in to open afterwards. The seeded chat is asked again once, so it holds a request and a failed turn.
func openedChatPage(t *testing.T, p *App, id string) (actions func() []search.Action, opened *[]string) {
	t.Helper()
	m := p.extensionManager()
	result, failure, err := m.Run(context.Background(), aichats.ID, api.Invocation{Ref: aichats.OpenChatRef(id)})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("open chat: view=%v failure=%v err=%v", result.View != nil, failure, err)
	}
	deps := p.extensionViewDeps(m)
	var chatIDs []string
	deps.Invoke = func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) {
		chatIDs = append(chatIDs, in.Ref.Args["chatId"])
	}
	page := extview.NewSession(deps, aichats.ID, *result.View).Palette()
	return page.Actions, &chatIDs
}

func TestRetryForksTheChatThroughThePalettePageAndKeepsTheOriginal(t *testing.T) {
	p := goldenPalette(t)
	if err := p.chats.Regenerate("c-release"); err != nil {
		t.Fatal(err)
	}
	before, _ := p.chats.Chat("c-release")
	if len(before.Messages) != 2 || before.Messages[1].Author != chat.AuthorNotice {
		t.Fatalf("seeded transcript = %+v, want the request and the failed turn", before.Messages)
	}
	actions, opened := openedChatPage(t, p, "c-release")
	var retry *search.Action
	for _, action := range actions() {
		if strings.HasPrefix(action.Title, "Retry message 2") {
			retry = &action
			if action.Label != "" {
				t.Errorf("Retry brought the default shortcut %q", action.Label)
			}
		}
	}
	if retry == nil {
		t.Fatal("the failed turn has no Retry row")
	}
	host := &recordingHost{}
	retry.Run(host)
	chats := p.chats.Chats()
	if len(chats) != 3 || len(host.toasts) != 0 {
		t.Fatalf("chats = %d, toasts = %q", len(chats), host.toasts)
	}
	after, _ := p.chats.Chat("c-release")
	if len(after.Messages) != len(before.Messages) || after.Messages[1].Author != chat.AuthorNotice || after.Messages[0].Text != before.Messages[0].Text {
		t.Fatalf("the original changed: %+v", after.Messages)
	}
	if len(*opened) != 1 || (*opened)[0] == "c-release" || (*opened)[0] == "" {
		t.Fatalf("the palette opened %v, want the new chat", *opened)
	}
	fork, ok := p.chats.Chat((*opened)[0])
	if !ok || fork.Kind != chat.KindConversation || fork.Extension != "" || fork.Messages[0].Text != before.Messages[0].Text {
		t.Fatalf("fork = %+v", fork)
	}
}

func TestRetryIsRefusedWhenTheStoreChangedAfterTheMenuOpened(t *testing.T) {
	p := goldenPalette(t)
	if err := p.chats.Regenerate("c-release"); err != nil {
		t.Fatal(err)
	}
	actions, opened := openedChatPage(t, p, "c-release")
	var retry search.Action
	for _, action := range actions() {
		if strings.HasPrefix(action.Title, "Retry message 2") {
			retry = action
		}
	}
	if err := p.chats.Send("c-release", "one more question"); err != nil {
		t.Fatal(err)
	}
	host := &recordingHost{}
	retry.Run(host)
	if len(p.chats.Chats()) != 2 || len(*opened) != 0 || len(host.toasts) != 1 || host.toasts[0] != chat.ErrStaleFork.Error() {
		t.Fatalf("chats = %d, opened %v, toasts %q", len(p.chats.Chats()), *opened, host.toasts)
	}
}

func TestRetryIsRefusedForABuilderChat(t *testing.T) {
	p := goldenPalette(t)
	builder, err := p.chats.NewChat(chat.KindBuilder, chat.TierSmart)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (extensionChatPages{p: p}).Retry(builder.ID, 1, nil); !errors.Is(err, chat.ErrBuilderFork) {
		t.Fatalf("retry of a builder chat = %v, want it refused", err)
	}
}
