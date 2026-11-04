package aichat

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// TestMenuSwitchShowsTheNewTierOnThePicker checks the menu's tier switch moves the picker's label to the other tier.
func TestMenuSwitchShowsTheNewTierOnThePicker(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Tier label", chat.TierFast, user("q"), assistant("a"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if !tt.HasText("Fast ⌄") {
		t.Fatal("the picker does not name the Fast tier before the switch")
	}

	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Switch to Smart")
	tt.Frame()
	if !tt.HasText("Smart ⌄") {
		t.Fatal("the picker does not name the Smart tier after the switch")
	}
}

// TestSwitchWhileAReplyStreamsSaysWhy checks the tier does not move while a reply streams, and the palette says so.
func TestSwitchWhileAReplyStreamsSaysWhy(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Busy tier", chat.TierFast, user("q"))
	if err := r.service.Send(record.ID, "still going"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	chooseMenuAction(t, r, tt, view, "Switch to Smart")
	tt.Frame()
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierFast {
		t.Fatalf("tier = %s while a reply streams, want it unchanged", got)
	}
	if !r.said(toastStillAnswering) {
		t.Fatal("the palette does not say why the tier stayed")
	}
}
