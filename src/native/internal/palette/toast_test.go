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
