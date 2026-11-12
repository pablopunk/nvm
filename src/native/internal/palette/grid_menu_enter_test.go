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

// TestKeysTypedBeforeTheFrameThatClosesTheMenuReachThePageInOrder delivers Escape and Enter to the menu's field as
// they arrive, with no frame between them, as a driver types them: Escape queued as a shortcut would close the menu
// only in the next frame, after the field took Enter for the menu and the menu went away with it. Escape closes the
// menu as it arrives, so Enter is the page's: the chosen cell's primary runs.
func TestKeysTypedBeforeTheFrameThatClosesTheMenuReachThePageInOrder(t *testing.T) {
	for _, closing := range []ui.TextKey{{Key: ui.KeyEscape}, {Key: ui.KeyK, Mods: Mod}} {
		var ran []string
		h := newHarness(t, gridView("Root", &ran, gridTitles...), true)
		h.key(0, ui.KeyRight)
		h.key(Mod, ui.KeyK)
		if !h.p.menuTextKey(closing) || h.p.actionsOpen {
			t.Fatalf("%v did not close the menu as it arrived", closing.Key)
		}
		if !h.p.menuTextKey(ui.TextKey{Key: ui.KeyEnter}) || !slices.Equal(ran, []string{"B"}) {
			t.Fatalf("Enter typed right after %v ran %v, want [B]", closing.Key, ran)
		}
	}
}
