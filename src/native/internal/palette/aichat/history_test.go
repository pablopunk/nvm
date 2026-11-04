package aichat

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
)

// titledChat creates a chat of kind named title, dated against goldenNow, and reloads the service.
func (r *rig) titledChat(kind chat.Kind, title string) chat.Chat {
	r.t.Helper()
	return r.seedChat(kind, chat.TierSmart, title, "", []chat.Message{user("hello")})
}

// TestHistoryListsBuilderChatsButNotProofreads checks the AI Chats list shows conversations and builder chats, and
// keeps a proofread out, as the desktop list does.
func TestHistoryListsBuilderChatsButNotProofreads(t *testing.T) {
	r := newRig(t, nil)
	r.titledChat(chat.KindConversation, "Plan a weekend")
	r.titledChat(chat.KindBuilder, "Clock widget")
	r.titledChat(chat.KindProofread, "Fix this sentence")

	items := historyRows(r.deps(), "")
	titles := resultTitles(items)
	if !contains(titles, "Plan a weekend") || !contains(titles, "Clock widget") {
		t.Fatalf("history = %v, want the conversation and the builder chat", titles)
	}
	if contains(titles, "Fix this sentence") {
		t.Fatalf("history = %v, must not list a proofread", titles)
	}
	for _, item := range items {
		if item.Title == "Clock widget" && !strings.HasPrefix(item.Subtitle, "AI builder chat") {
			t.Fatalf("builder row subtitle = %q, want it to name the builder chat", item.Subtitle)
		}
	}
}

// TestOpeningABuilderChatFromHistoryResumesIt checks a builder chat in the list opens its transcript, so the user can
// carry on with it.
func TestOpeningABuilderChatFromHistoryResumesIt(t *testing.T) {
	r := newRig(t, nil)
	r.titledChat(chat.KindBuilder, "Clock widget")
	r.palette.Show()
	r.open(HistoryView(r.deps()))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	top := r.palette.Top()
	if top == nil || top.TitleFn == nil || top.TitleFn() != "Clock widget" {
		t.Fatalf("top page = %v, want the builder chat's page", top)
	}
	if !contains(resultTitles(historyRows(r.deps(), "")), "New Chat") {
		t.Fatal("the list lost its new-chat row")
	}
}

// TestChatRowMenuSaysRemoveChatAndConfirmsWithTheHistory checks a chat row in AI Chats offers Remove Chat, not Remove
// action, and that the confirmation names the chat and its history, with Remove Chat as its first row.
func TestChatRowMenuSaysRemoveChatAndConfirmsWithTheHistory(t *testing.T) {
	r := newRig(t, nil)
	record := r.titledChat(chat.KindConversation, "Trip ideas")
	r.palette.Show()
	r.open(HistoryView(r.deps()))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	chooseDelete(r, tt)
	top := r.palette.Top()
	if top == nil || top.Title != "Confirm" {
		t.Fatalf("top page = %v after Remove Chat, want the confirmation", top)
	}
	if want := `Remove "Trip ideas" and its history?`; top.Placeholder != want {
		t.Fatalf("confirmation = %q, want %q", top.Placeholder, want)
	}
	if titles := resultTitles(top.Search("")); len(titles) < 2 || titles[0] != "Remove Chat" || titles[1] != "Cancel" {
		t.Fatalf("confirmation rows = %v, want Remove Chat then Cancel", titles)
	}
	tt.Key(0, ui.KeyEnter)
	if _, ok := r.service.Chat(record.ID); ok {
		t.Fatal("Remove Chat in the confirmation kept the chat")
	}
}

// TestNewChatLeavesNoHistoryRowUntilItsFirstMessage checks opening a new chat records nothing: the page is shown, and
// the list keeps only its New Chat row.
func TestNewChatLeavesNoHistoryRowUntilItsFirstMessage(t *testing.T) {
	r := newRig(t, nil)
	view := newChat(r.deps())
	if view == nil || view.Title != "AI Chat" {
		t.Fatalf("new chat page = %v, want an AI Chat page", view)
	}
	if rows := historyRows(r.deps(), ""); len(rows) != 1 || rows[0].Title != "New Chat" {
		t.Fatalf("history rows = %v after New Chat, want only New Chat", resultTitles(rows))
	}
	if stored, _ := r.store.All(); len(stored) != 0 || len(r.service.Chats()) != 0 {
		t.Fatalf("stored = %v, listed = %v after New Chat, want none", stored, r.service.Chats())
	}
}

// TestEmptyChatsSavedBeforeAreNotListed checks a chat an older build saved with no message stays out of the history.
func TestEmptyChatsSavedBeforeAreNotListed(t *testing.T) {
	r := newRig(t, nil)
	if err := r.store.Insert(chat.Chat{ID: "chat-old-empty", Kind: chat.KindConversation, Tier: chat.TierFast, UpdatedAt: goldenNow, CreatedAt: goldenNow}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	r.service.Reload()
	if rows := historyRows(r.deps(), ""); len(rows) != 1 {
		t.Fatalf("history rows = %v, want only New Chat", resultTitles(rows))
	}
}
