package aichat

import (
	"slices"
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
	if !tt.HasText("Fast") {
		t.Fatal("the picker does not name the Fast tier before the switch")
	}

	tt.Key(palette.Mod, ui.KeyK)
	tt.Frame()
	chooseMenuAction(t, r, tt, view, "Switch to Smart")
	tt.Frame()
	if !tt.HasText("Smart") {
		t.Fatal("the picker does not name the Smart tier after the switch")
	}
}

// TestTierPickerOpensANativeMenuOfBothTiers checks a press on the picker opens the menu of the tiers, Fast then Smart.
func TestTierPickerOpensANativeMenuOfBothTiers(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Tier menu", chat.TierFast, user("q"), assistant("a"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Fast"); err != nil {
		t.Fatalf("click the picker: %v", err)
	}
	tt.Frame()
	if got := tt.Menu(); !slices.Equal(got, []string{"Fast", "Smart"}) {
		t.Fatalf("menu = %v, want Fast and Smart", got)
	}
}

// TestChoosingATierInTheMenuSwitchesIt checks the tier chosen in the menu becomes the chat's tier, and the palette says so.
func TestChoosingATierInTheMenuSwitchesIt(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Tier choice", chat.TierFast, user("q"), assistant("a"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Fast"); err != nil {
		t.Fatalf("click the picker: %v", err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("Smart"); err != nil {
		t.Fatalf("choose Smart: %v", err)
	}
	tt.Frame()
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierSmart {
		t.Fatalf("tier = %s after choosing Smart, want smart", got)
	}
	if !r.said("Next replies use the Smart model.") {
		t.Fatal("the palette does not say the next replies use Smart")
	}
	if !tt.HasText("Smart") {
		t.Fatal("the picker does not name the Smart tier after the choice")
	}
}

// TestClosingTheTierMenuChangesNothing checks Escape, or a click elsewhere, closes the menu and keeps the tier.
func TestClosingTheTierMenuChangesNothing(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Tier closed", chat.TierFast, user("q"), assistant("a"))
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Fast"); err != nil {
		t.Fatalf("click the picker: %v", err)
	}
	tt.Frame()
	tt.CloseMenu()
	tt.Frame()
	if tt.Menu() != nil {
		t.Fatal("the menu is still shown after it closed")
	}
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierFast {
		t.Fatalf("tier = %s after closing the menu, want fast", got)
	}
	if r.said("Next replies use") {
		t.Fatal("the palette reports a tier change after the menu closed with no choice")
	}
}

// TestTierPickerIsDisabledWhileAReplyStreams checks a press on the picker opens no menu while a reply streams.
func TestTierPickerIsDisabledWhileAReplyStreams(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Tier busy", chat.TierFast, user("q"))
	if err := r.service.Send(record.ID, "still going"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	view := ChatView(r.deps(), record.ID)
	r.open(view)
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	_ = tt.Click("Fast")
	tt.Frame()
	if tt.Menu() != nil {
		t.Fatal("the tier menu opened while a reply streams")
	}
	if got := mustChat(t, r, record.ID).Tier; got != chat.TierFast {
		t.Fatalf("tier = %s while a reply streams, want it unchanged", got)
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
