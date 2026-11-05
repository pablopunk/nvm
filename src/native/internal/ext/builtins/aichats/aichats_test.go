package aichats

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeChats is the host's chat store as the built-in reads it.
type fakeChats struct {
	api.AIChats
	chats []api.ChatDetail
}

func (c fakeChats) List() []api.ChatSummary {
	out := make([]api.ChatSummary, 0, len(c.chats))
	for _, chat := range c.chats {
		out = append(out, chat.ChatSummary)
	}
	return out
}

func (c fakeChats) Get(id string) (api.ChatDetail, bool) {
	for _, chat := range c.chats {
		if chat.ID == id {
			return chat, true
		}
	}
	return api.ChatDetail{}, false
}

type fakeAI struct {
	api.AI
	chats fakeChats
}

func (a fakeAI) Chats() api.AIChats { return a.chats }

// fakeBuilder records the cleanup and the tweak the built-in asks for.
type fakeBuilder struct {
	api.AIBuilder
	cleaned int
}

func (b *fakeBuilder) StartChat(prompt, title string) api.Action {
	return api.Action{Kind: api.ActionBuilder, Op: api.BuilderStart, Builder: &api.BuilderAction{Prompt: prompt}}
}

func (b *fakeBuilder) CleanupExpiredConversations() (int, error) {
	b.cleaned++
	return 0, nil
}

func (b *fakeBuilder) TweakExtension(request api.TweakRequest) api.Action {
	return api.Action{Kind: api.ActionBuilder, Op: api.BuilderTweak, Builder: &api.BuilderAction{ExtensionID: request.ExtensionID}}
}

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

// fakeNavigation answers a push with the view it was given, as the host would show it.
type fakeNavigation struct {
	api.Navigation
}

func (fakeNavigation) Push(view *api.View) api.Result { return api.Result{View: view} }

type fakeContext struct {
	api.Context
	chats   fakeChats
	builder *fakeBuilder
	now     time.Time
}

func (c fakeContext) AI() api.AI               { return fakeAI{chats: c.chats} }
func (c fakeContext) AIBuilder() api.AIBuilder { return c.builder }
func (c fakeContext) Clock() api.Clock         { return fakeClock{now: c.now} }
func (c fakeContext) Navigation() api.Navigation {
	return fakeNavigation{}
}

var testNow = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func contextWith(chats ...api.ChatDetail) fakeContext {
	return fakeContext{chats: fakeChats{chats: chats}, builder: &fakeBuilder{}, now: testNow}
}

func chat(id, kind, title string, messages ...api.ChatMessage) api.ChatDetail {
	return api.ChatDetail{ChatSummary: api.ChatSummary{ID: id, Kind: kind, Title: title, UpdatedAtMs: testNow.UnixMilli()}, Messages: messages}
}

func TestContinueRowsKeepTheFourMostRecentListedChats(t *testing.T) {
	ctx := contextWith(
		chat("1", "conversation", "Alpha"), chat("2", "proofread", "Proof"), chat("3", "builder", "Beta"),
		chat("4", "conversation", ""), chat("5", "conversation", "Gamma"), chat("6", "conversation", "Delta"),
		chat("7", "conversation", "Epsilon"),
	)
	items, err := Extension{}.RootItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Continue conversation: Alpha", "Continue AI chat: Beta", "Continue conversation: Gamma", "Continue conversation: Delta"}
	if len(items) != len(want) {
		t.Fatalf("root rows = %d, want %d", len(items), len(want))
	}
	for i, item := range items {
		if item.Title != want[i] {
			t.Fatalf("row %d = %q, want %q", i, item.Title, want[i])
		}
	}
}

func TestContinueRowsFilterByTitleWhenTyped(t *testing.T) {
	ctx := contextWith(chat("1", "conversation", "Release notes"), chat("2", "conversation", "Time zones"))
	items, err := Extension{}.SearchItems(ctx, "release")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Title != "Continue conversation: Release notes" {
		t.Fatalf("search rows = %+v, want the release chat and the Automate fallback", items)
	}
	if !items[1].QueryFallback || !strings.HasPrefix(items[1].Title, "Automate") {
		t.Fatalf("last row = %+v, want the Automate fallback", items[1])
	}
}

func TestAutomateIsNotOfferedForAWebsite(t *testing.T) {
	items, err := Extension{}.SearchItems(contextWith(), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.QueryFallback {
			t.Fatalf("a website query got the Automate row: %+v", item)
		}
	}
}

func TestHistoryListLeadsWithNewChatAndAsksBeforeRemoving(t *testing.T) {
	ctx := contextWith(chat("1", "conversation", "Alpha"), chat("2", "proofread", "Proof"))
	view := historyView(ctx)
	if len(view.Items) != 2 || view.Items[0].Title != "New Chat" {
		t.Fatalf("history rows = %+v, want New Chat and one listed chat", view.Items)
	}
	remove := view.Items[1].Actions[1]
	if remove.Op != api.ChatRemove || remove.Confirm == nil || !remove.Confirm.Destructive {
		t.Fatalf("remove action = %+v, want a destructive chat remove with a confirmation", remove)
	}
}

func TestSubmitBecomesAContinueWithTextAndPictures(t *testing.T) {
	in := api.Invocation{
		Ref:    api.ActionRef{ID: opSubmit, Args: map[string]string{argChatID: "1"}},
		Form:   api.FormValues{"message": {Text: "hello"}},
		Images: []api.ChatImage{{Title: "shot.png", DataURL: "data:image/png;base64,AA=="}},
	}
	result, err := Extension{}.RunAction(contextWith(chat("1", "conversation", "Alpha")), in)
	if err != nil {
		t.Fatal(err)
	}
	if result.Next == nil || result.Next.Op != api.ChatContinue {
		t.Fatalf("result = %+v, want a continue action", result)
	}
	if result.Next.Chat.ChatID != "1" || result.Next.Chat.Message != "hello" || len(result.Next.Chat.Images) != 1 {
		t.Fatalf("continue payload = %+v", result.Next.Chat)
	}
}

func TestStreamingChatRefreshesOnATickUntilTheReplyEnds(t *testing.T) {
	streaming := chat("1", "conversation", "Alpha", api.ChatMessage{Role: "assistant", Content: "Part", Streaming: true})
	if view := chatView(streaming); view.Refresh == nil || view.Refresh.IntervalMs != streamTickMs {
		t.Fatalf("a streaming chat must refresh on a tick, got %+v", view.Refresh)
	}
	done := chat("2", "conversation", "Beta", api.ChatMessage{Role: "assistant", Content: "All done."})
	if view := chatView(done); view.Refresh != nil {
		t.Fatalf("a finished chat must not refresh, got %+v", view.Refresh)
	}
}

func TestAIChatsCommandSubtitleCountsTheChats(t *testing.T) {
	commands, err := Extension{}.Commands(contextWith(chat("1", "conversation", "A"), chat("2", "builder", "B")))
	if err != nil {
		t.Fatal(err)
	}
	if commands[0].ID != CommandAIChats || commands[0].Subtitle != "2 AI chats" {
		t.Fatalf("AI Chats command = %+v", commands[0])
	}
}

func TestCleanupCommandRunsTheBuilderCleanup(t *testing.T) {
	ctx := contextWith()
	if _, err := (Extension{}).RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: cleanupCommandID}}); err != nil {
		t.Fatal(err)
	}
	if ctx.builder.cleaned != 1 {
		t.Fatalf("cleanups = %d, want 1", ctx.builder.cleaned)
	}
}

func TestUnknownActionIsRejected(t *testing.T) {
	_, err := Extension{}.RunAction(contextWith(), api.Invocation{Ref: api.ActionRef{ID: "nope"}})
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestOpeningAGoneChatFails(t *testing.T) {
	_, err := Extension{}.RunAction(contextWith(), api.Invocation{Ref: api.ActionRef{ID: opOpenChat, Args: map[string]string{argChatID: "missing"}}})
	if !errors.Is(err, errChatGone) {
		t.Fatalf("err = %v, want errChatGone", err)
	}
}
