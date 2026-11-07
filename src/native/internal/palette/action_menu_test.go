package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestClickingAMenuRowRunsItsAction checks a mouse click on a row of the open action menu runs that action and closes
// the menu, as Enter does for the chosen row.
func TestClickingAMenuRowRunsItsAction(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Clicks", &ran, "Alpha"), true)
	h.key(Mod, ui.KeyK)
	if !h.p.actionsOpen {
		t.Fatal("⌘K did not open the action menu")
	}

	if err := h.tt.Click("Secondary"); err != nil {
		t.Fatalf("click the menu row: %v", err)
	}
	h.frame()
	if len(ran) != 1 || ran[0] != "menu:Alpha" {
		t.Fatalf("ran = %v after the click, want the Secondary action of Alpha", ran)
	}
	if h.p.actionsOpen {
		t.Fatal("the menu stayed open after a row was clicked")
	}
}

// TestHoveringAMenuRowChoosesIt checks the row under the pointer becomes the chosen one, so Enter runs what the pointer
// rests on.
func TestHoveringAMenuRowChoosesIt(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Hovers", &ran, "Alpha"), true)
	h.key(Mod, ui.KeyK)
	if h.p.actChosen != 0 {
		t.Fatalf("the menu opens on row %d, want the first", h.p.actChosen)
	}

	rect, ok := h.tt.Find("Secondary")
	if !ok {
		t.Fatal("the menu shows no Secondary row")
	}
	h.tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	h.frame()
	if h.p.actChosen != 1 {
		t.Fatalf("chosen row = %d after the pointer rests on Secondary, want 1", h.p.actChosen)
	}
	h.key(0, ui.KeyEnter)
	if len(ran) != 1 || ran[0] != "menu:Alpha" {
		t.Fatalf("ran = %v after Enter on the hovered row, want the Secondary action", ran)
	}
}
