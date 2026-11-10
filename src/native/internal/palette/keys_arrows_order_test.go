package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/search"
)

func arrowRows(ran *string) []search.Item {
	var rows []search.Item
	for _, title := range []string{"First", "Second", "Third", "Fourth"} {
		rows = append(rows, search.Item{ID: title, Title: title, Actions: []search.Action{{
			Title: "Run", Dismiss: search.DismissNever, Run: func(search.Host) { *ran = title },
		}}})
	}
	return rows
}

// A frame can be late, so several keys can arrive before the next one is drawn. They must all count, in order.
func TestArrowsPressedBeforeTheNextFrameAllCountAndKeepTheirOrderWithEnter(t *testing.T) {
	var ran string
	view := &View{Title: "Page", Search: func(string) []search.Item { return arrowRows(&ran) }}
	h := newHarness(t, view, false)
	h.frame()
	for range 2 {
		if !h.p.stepWithArrow(ui.KeyDown) {
			t.Fatal("the arrow was left for the next frame")
		}
	}
	h.p.onEnter()
	if ran != "Third" {
		t.Fatalf("Enter ran %q, want the row two below the first", ran)
	}
	h.p.stepWithArrow(ui.KeyUp)
	if got := h.chosenTitle(); got != "Second" {
		t.Fatalf("after Up the choice is %q", got)
	}
}

func TestOneArrowKeyMovesOneRow(t *testing.T) {
	var ran string
	view := &View{Title: "Page", Search: func(string) []search.Item { return arrowRows(&ran) }}
	h := newHarness(t, view, false)
	h.frame()
	h.key(0, ui.KeyDown)
	if got := h.chosenTitle(); got != "Second" {
		t.Fatalf("one Down chose %q; the key must not also count in the frame", got)
	}
}

func TestArrowsStepWholeGridRowsAndLeaveBodiesAlone(t *testing.T) {
	var ran string
	view := &View{Title: "Grid", Grid: &Grid{Columns: 2, Cell: func(*ui.Context, search.Item) {}}, Search: func(string) []search.Item { return arrowRows(&ran) }}
	h := newHarness(t, view, false)
	h.frame()
	h.p.stepWithArrow(ui.KeyDown)
	if got := h.chosenTitle(); got != "Third" {
		t.Fatalf("Down in a two-column grid chose %q", got)
	}
	h.p.cur().Grid = nil
	h.p.cur().Body = func(*ui.Context, search.Host) {}
	if h.p.stepWithArrow(ui.KeyDown) {
		t.Fatal("a page that draws its own body lost its arrow")
	}
}
