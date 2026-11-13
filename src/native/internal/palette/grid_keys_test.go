package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// gridView is a sub-view whose rows are laid out as three columns, the shape of the emoji picker. Its rows keep the
// actions rowsView gives them, so a test can tell whether a key reached the list's own menu shortcut.
func gridView(title string, ran *[]string, titles ...string) *View {
	v := rowsView(title, ran, titles...)
	v.Grid = &Grid{Columns: 3, CellHeight: 40, Cell: func(*ui.Context, search.Item) {}}
	return v
}

// gridTitles are the rows of the grid tests, seven in three columns: the second row holds D, E and F, and G starts the
// third.
var gridTitles = []string{"A", "B", "C", "D", "E", "F", "G"}

// TestGridArrowsMoveTheChoiceAcrossCellsAndRows walks the four arrows from the first cell with the field empty:
// Right and Left step across a row, Down and Up step by a row of cells.
func TestGridArrowsMoveTheChoiceAcrossCellsAndRows(t *testing.T) {
	var ran []string
	h := newHarness(t, gridView("Root", &ran, gridTitles...), true)
	steps := []struct {
		key  ui.Key
		want string
	}{
		{ui.KeyRight, "B"},
		{ui.KeyDown, "E"},
		{ui.KeyLeft, "D"},
		{ui.KeyUp, "A"},
	}
	for _, step := range steps {
		h.key(0, step.key)
		if got := h.chosenTitle(); got != step.want {
			t.Fatalf("after %v the grid chose %q, want %q", step.key, got, step.want)
		}
	}
}

// TestGridArrowsWrapFromOneRowToTheNextAndStopAtTheEnds checks the 1.x rule: Right at the end of a row moves to the
// first cell of the next row, and Left at the start of a row moves to the last cell of the row above. At the first and
// last cell of the grid the choice stops, as 1.x clamps it.
func TestGridArrowsWrapFromOneRowToTheNextAndStopAtTheEnds(t *testing.T) {
	var ran []string
	h := newHarness(t, gridView("Root", &ran, gridTitles...), true)

	h.key(0, ui.KeyRight)
	h.key(0, ui.KeyRight)
	if got := h.chosenTitle(); got != "C" {
		t.Fatalf("two Right presses chose %q, want C at the end of the first row", got)
	}
	h.key(0, ui.KeyRight)
	if got := h.chosenTitle(); got != "D" {
		t.Fatalf("Right at the end of a row chose %q, want D, the first cell of the next row", got)
	}
	h.key(0, ui.KeyLeft)
	if got := h.chosenTitle(); got != "C" {
		t.Fatalf("Left at the start of a row chose %q, want C, the last cell of the row above", got)
	}

	for range 2 {
		h.key(0, ui.KeyUp)
	}
	h.key(0, ui.KeyLeft)
	h.key(0, ui.KeyLeft)
	if got := h.chosenTitle(); got != "A" {
		t.Fatalf("Up and Left past the first cell chose %q, want A (no wrap)", got)
	}

	for range 3 {
		h.key(0, ui.KeyDown)
	}
	h.key(0, ui.KeyRight)
	h.key(0, ui.KeyRight)
	h.key(0, ui.KeyRight)
	if got := h.chosenTitle(); got != "G" {
		t.Fatalf("Right and Down past the last cell chose %q, want G (no wrap)", got)
	}
}

// TestGridArrowsMoveTheGridNotTheCaretWhileTheFieldHoldsText types into the field, then moves with the arrows: the
// query is unchanged and the choice moves, and a character typed afterwards lands at the end of the query, where the
// caret was, so the arrows never moved it.
func TestGridArrowsMoveTheGridNotTheCaretWhileTheFieldHoldsText(t *testing.T) {
	var ran []string
	h := newHarness(t, gridView("Root", &ran, gridTitles...), true)
	h.typeText("ab")

	h.key(0, ui.KeyRight)
	if got := h.chosenTitle(); got != "B" {
		t.Fatalf("Right with text in the field chose %q, want B", got)
	}
	h.key(0, ui.KeyLeft)
	h.key(0, ui.KeyLeft)
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyUp)
	if got := h.chosenTitle(); got != "A" {
		t.Fatalf("Left, Left, Down, Up with text in the field chose %q, want A", got)
	}
	if h.p.Query() != "ab" {
		t.Fatalf("the arrows changed the query to %q, want ab", h.p.Query())
	}

	h.typeText("x")
	if h.p.Query() != "abx" {
		t.Fatalf("typing after the arrows made the query %q, want abx: an arrow moved the caret", h.p.Query())
	}
}

// TestGridLeftAndRightNeverOpenTheMenuOrGoBack checks that a grid keeps the arrows Left and Right for its cells: at the
// end of the query Right does not open the action menu, and at the start Left does not leave the page, as they do in
// a list.
func TestGridLeftAndRightNeverOpenTheMenuOrGoBack(t *testing.T) {
	var ran []string
	root := rowsView("Root", &ran, "Alpha")
	h := newHarness(t, root, true)
	grid := gridView("Grid", &ran, gridTitles...)
	h.p.Push(grid)

	h.key(0, ui.KeyRight)
	if h.p.actionsOpen {
		t.Fatal("Right at the end of the query opened the action menu of a grid")
	}
	h.key(0, ui.KeyLeft)
	h.key(0, ui.KeyLeft)
	if h.p.Top() != grid {
		t.Fatal("Left at the start of the query went back from a grid page")
	}
}
