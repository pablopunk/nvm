package palette

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEnterAfterClosingTheMenuWithEscapeRunsTheChosenCellsPrimary walks the Screenshots journey on a grid: move to
// the second cell, open the action menu, close it with Escape and press Enter. The chosen cell's primary action runs.
func TestEnterAfterClosingTheMenuWithEscapeRunsTheChosenCellsPrimary(t *testing.T) {
	var ran []string
	h := newHarness(t, gridView("Root", &ran, gridTitles...), true)
	h.key(0, ui.KeyRight)
	h.key(Mod, ui.KeyK)
	if !h.p.actionsOpen {
		t.Fatal("Command-K did not open the menu")
	}
	h.key(0, ui.KeyEscape)
	if h.p.actionsOpen {
		t.Fatal("Escape did not close the menu")
	}
	h.key(0, ui.KeyEnter)
	if !slices.Equal(ran, []string{"B"}) {
		t.Fatalf("Enter after closing the menu ran %v, want [B]", ran)
	}
}
