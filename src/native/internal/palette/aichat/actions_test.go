package aichat

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// chooseMenuAction runs the action titled title from the menu, the way the keyboard does: the row is chosen and Enter
// runs it.
func chooseMenuAction(t *testing.T, r *rig, tt *ui.Tester, view *palette.View, title string) {
	t.Helper()
	for i, a := range view.Actions() {
		if a.Title == title {
			r.palette.OpenActionMenu(i)
			tt.Key(0, ui.KeyEnter)
			return
		}
	}
	t.Fatalf("the menu has no %q", title)
}

// menuTitles lists the titles of the chat's action menu, in order.
func menuTitles(view *palette.View) []string {
	var titles []string
	for _, a := range view.Actions() {
		titles = append(titles, a.Title)
	}
	return titles
}

// TestChatMenuListsTheChatActionsInOrder checks the Command-K menu of an idle chat with a reply: Ask Again first, then
// the copies, the tier, the new chat and the removal.
func TestChatMenuListsTheChatActionsInOrder(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Plan", chat.TierFast, user("Plan a trip."), assistant("Three days in Lisbon."))
	view := ChatView(r.deps(), record.ID)
	r.open(view)

	want := []string{"Ask Again", "Copy Reply", "Copy Message", "Switch to Smart", "New Chat", "Delete Chat"}
	if got := menuTitles(view); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu = %v, want %v", got, want)
	}
}

// TestChatMenuWhileAReplyStreamsOffersStop checks Stop joins the menu while a reply streams, and Ask Again leaves it.
func TestChatMenuWhileAReplyStreamsOffersStop(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Busy", chat.TierFast, user("q"))
	if err := r.service.Send(record.ID, "tell me"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	view := ChatView(r.deps(), record.ID)
	r.open(view)

	want := []string{"Copy Reply", "Copy Message", "Stop", "Switch to Smart", "New Chat", "Delete Chat"}
	if got := menuTitles(view); !reflect.DeepEqual(got, want) {
		t.Fatalf("menu while busy = %v, want %v", got, want)
	}
}

// TestBuilderMenuOffersTheReview checks a builder chat that wrote an extension adds Review Extension last.
func TestBuilderMenuOffersTheReview(t *testing.T) {
	r := newRig(t, nil)
	deps := r.deps()
	deps.Review = func(string) *palette.View { return &palette.View{Title: "Review"} }
	record := r.builderWith("user.clock", user("Make a clock."), assistant("Done."), notice(installedNotice()))
	view := ChatView(deps, record.ID)
	r.open(view)

	titles := menuTitles(view)
	if len(titles) == 0 || titles[len(titles)-1] != "Review Extension" {
		t.Fatalf("menu = %v, want Review Extension last", titles)
	}
}

// TestCommandKShowsTheChatMenu checks ⌘K opens the menu on the page, with the chat's titles on it.
func TestCommandKShowsTheChatMenu(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Menu", chat.TierFast, user("q"), assistant("a"))
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if tt.HasText("Switch to Smart") {
		t.Fatal("the menu is open before ⌘K")
	}

	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	if !tt.HasText("Switch to Smart") || !tt.HasText("Copy Reply") {
		t.Fatal("⌘K did not show the chat's action menu")
	}
}

// TestMenuSwitchToSmartChangesTheTier checks the tier switch in the menu does what the picker does.
func TestMenuSwitchToSmartChangesTheTier(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Tier", chat.TierFast, user("q"), assistant("a"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Switch to Smart")
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierSmart {
		t.Fatalf("tier = %s after the menu switch, want smart", got)
	}
}

// TestMenuCopyMessageCopiesTheLastQuestion checks Copy Message copies the newest message the user sent.
func TestMenuCopyMessageCopiesTheLastQuestion(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Copy", chat.TierFast, user("first question"), assistant("an answer"), user("second question"), assistant("second answer"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Copy Message")
	if len(r.copied) != 1 || r.copied[0] != "second question" {
		t.Fatalf("copied = %q, want the newest question", r.copied)
	}
}

// TestMenuDeleteChatAsksFirst checks Delete Chat from the menu opens the question before anything is removed.
func TestMenuDeleteChatAsksFirst(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Delete", chat.TierFast, user("q"), assistant("a"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Delete Chat")
	if r.palette.Top().Title != "Confirm" {
		t.Fatalf("top page = %q after Delete Chat, want the confirmation", r.palette.Top().Title)
	}
	if _, ok := r.service.Chat(record.ID); !ok {
		t.Fatal("Delete Chat removed the chat before it was confirmed")
	}
}

// TestFooterNamesThePrimaryActionAndTheMenuKey checks the footer shows Send and the Actions key, as other views do,
// and no list of shortcuts.
func TestFooterNamesThePrimaryActionAndTheMenuKey(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Footer", chat.TierFast, user("q"), assistant("a"))
	r.open(ChatView(r.deps(), record.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if !tt.HasText("Send") || !tt.HasText("Actions") || !tt.HasText(palette.KeyLabel("⌃K")) {
		t.Fatal("the footer does not name Send and Actions ⌘K")
	}
	if tt.HasText("⌘R ask again · ⌘⌥C copy reply · ⌘⇧M model") {
		t.Fatal("the footer still lists the shortcuts as text")
	}
}
