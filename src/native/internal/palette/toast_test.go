package palette

import "testing"

// TestToastRaisedWhileHiddenIsDropped checks a message from a background job does not survive a hide: the palette
// shows only the messages of the presentation that is open.
func TestToastRaisedWhileHiddenIsDropped(t *testing.T) {
	h := newHarness(t, rowsView("Root", new([]string), "Alpha"), true)
	h.p.Toast("Quitting Fake Rig App…")
	h.p.Hide()
	h.p.Toast("Fake Rig App quit.")
	h.p.Show()
	h.frame()
	if h.p.toastMsg != "" {
		t.Fatalf("a toast raised while hidden reappeared on show: %q", h.p.toastMsg)
	}
}

// TestToastShowsOnTheOpenPalette checks a message raised while the palette is open reaches the next frame.
func TestToastShowsOnTheOpenPalette(t *testing.T) {
	h := newHarness(t, rowsView("Root", new([]string), "Alpha"), true)
	h.p.Toast("Quitting Fake Rig App…")
	h.frame()
	if h.p.toastMsg != "" {
		t.Fatalf("the open palette did not draw its toast: %q is still pending", h.p.toastMsg)
	}
}

// TestToastRaisedWhileHiddenGoesToFeedback checks a message from a background job while the palette is hidden reaches the
// feedback HUD, which outlives the hide, instead of being dropped.
func TestToastRaisedWhileHiddenGoesToFeedback(t *testing.T) {
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
