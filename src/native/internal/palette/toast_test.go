package palette

import "testing"

// TestToastGoesToTheHUDWhileThePaletteIsUp pins that a message raised while the palette is open reaches the HUD, not
// a line under the search field, as 1.x shows every action result in its HUD.
func TestToastGoesToTheHUDWhileThePaletteIsUp(t *testing.T) {
	var fed []Feedback
	h := newHarness(t, rowsView("Root", new([]string), "Alpha"), true, func(o *Options) {
		o.Feedback = func(f Feedback) { fed = append(fed, f) }
	})
	h.p.Toast("Quitting Fake Rig App…")
	h.frame()
	if len(fed) != 1 || fed[0].Message != "Quitting Fake Rig App…" {
		t.Fatalf("the open palette sent %+v to the HUD, want its message", fed)
	}
}

// TestToastRaisedWhileHiddenGoesToTheHUD checks a message from a background job while the palette is hidden reaches the
// HUD, which outlives the hide, instead of being dropped.
func TestToastRaisedWhileHiddenGoesToTheHUD(t *testing.T) {
	var fed []Feedback
	h := newHarness(t, rowsView("Root", new([]string), "Alpha"), true, func(o *Options) {
		o.Feedback = func(f Feedback) { fed = append(fed, f) }
	})
	h.p.Hide()
	h.p.Toast("Paste failed. The item is on the clipboard.")
	if len(fed) != 1 || fed[0].Message != "Paste failed. The item is on the clipboard." {
		t.Fatalf("feedback got %+v while the palette was hidden", fed)
	}
}
