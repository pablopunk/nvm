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
	record, err := r.service.NewChat(kind, chat.TierSmart)
	if err != nil {
		r.t.Fatalf("NewChat: %v", err)
	}
	if err := r.store.Mutate(record.ID, goldenNow, func(c *chat.Chat) { c.Title = title }); err != nil {
		r.t.Fatalf("title chat: %v", err)
	}
	r.service.Reload()
	record, _ = r.service.Chat(record.ID)
	return record
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
