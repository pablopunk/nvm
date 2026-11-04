package palette

import (
	"testing"
)

// blurOnce simulates the window losing focus: the host's blur handler calls DismissOnBlur.
func blurOnce(h *harness) {
	h.p.DismissOnBlur()
	h.clock.AwaitTimers(1)
}

// passGrace lets the dismissal's delay pass and runs what the window queued for it.
func passGrace(t *testing.T, h *harness) {
	t.Helper()
	h.clock.Advance(blurDismissGrace)
	h.win.awaitQueued(t)
	h.win.drain()
	h.tt.Frame()
}

// TestBlurHidesThePaletteAfterTheGrace checks a click away: the focus loss hides the palette once the grace passes.
func TestBlurHidesThePaletteAfterTheGrace(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	blurOnce(h)
	passGrace(t, h)
	if h.win.IsVisible() {
		t.Fatal("the palette is still shown after a focus loss and the grace")
	}
}

// TestSummonPressDuringTheGraceHidesThePalette checks the summon shortcut: its own grab reports a focus loss, and
// the press that follows hides the palette once, not hides and then shows it again.
func TestSummonPressDuringTheGraceHidesThePalette(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	blurOnce(h)
	h.p.Toggle()
	passGrace(t, h)
	if h.win.IsVisible() {
		t.Fatal("the summon press did not hide the palette")
	}
	if h.win.hides != 1 {
		t.Fatalf("the palette was hidden %d times, want once", h.win.hides)
	}
}

// TestShowingAgainWithinTheGraceKeepsThePalette checks that a palette shown again during the grace is not
// dismissed by the earlier focus loss.
func TestShowingAgainWithinTheGraceKeepsThePalette(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	blurOnce(h)
	h.p.Hide()
	h.p.Show()
	passGrace(t, h)
	if !h.win.IsVisible() {
		t.Fatal("the earlier focus loss hid the palette shown again")
	}
}
