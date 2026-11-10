package palette

import (
	"testing"
)

// TestPopRevealsThePageBelow: a page whose rows come from outside the palette runs its OnReveal when a page above it
// is popped, so it shows what changed while it was covered.
func TestPopRevealsThePageBelow(t *testing.T) {
	var ran []string
	root := rowsView("Root", &ran, "Alpha")
	revealed := 0
	root.OnReveal = func() { revealed++ }
	h := newHarness(t, root, true)
	h.p.Push(rowsView("Child", &ran, "Beta"))
	h.frame()
	if revealed != 0 {
		t.Fatalf("pushing a page revealed the page below %d times", revealed)
	}
	h.p.Pop()
	if revealed != 1 {
		t.Fatalf("popping revealed the page below %d times, want once", revealed)
	}
}
