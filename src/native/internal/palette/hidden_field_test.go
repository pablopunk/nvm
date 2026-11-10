package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestReturnRunsTheSelectedRowOfAViewThatHidesItsField pushes a list view with no search field, as the extension review
// does, and presses Return. The row must run: with the field hidden the keyboard has to go to the list.
func TestReturnRunsTheSelectedRowOfAViewThatHidesItsField(t *testing.T) {
	root := &View{Title: "Root", Search: func(string) []search.Item { return nil }}
	h := newHarness(t, root, true)
	var ran []string
	h.p.Push(&View{Title: "Review", HideInput: true, Search: func(string) []search.Item {
		return []search.Item{{Title: "Enable", Actions: []search.Action{{Title: "Enable", Dismiss: search.DismissNever,
			Run: func(search.Host) { ran = append(ran, "enable") }}}}}
	}})
	h.frame()
	h.key(0, ui.KeyEnter)
	if len(ran) != 1 {
		t.Fatalf("Return ran %v; want the selected row to run once", ran)
	}
}

// TestSearchFieldKeepsItsLabelAndFocusWhenTheCaretMoves replaces the query as a form edit does, then types. The field
// must still be the labelled, focused search field, so assistive technology and the keyboard both find it.
func TestSearchFieldKeepsItsLabelAndFocusWhenTheCaretMoves(t *testing.T) {
	root := &View{Title: "Root", Search: func(string) []search.Item { return nil }}
	h := newHarness(t, root, true)

	h.p.SetQuery("A")
	h.frame()
	if !h.tt.Focused("Search") {
		t.Fatal("after the query was replaced the search field lost its label or its focus")
	}
	h.typeText("da")
	if !h.tt.Focused("Search") || h.p.Query() != "Ada" {
		t.Fatalf("after typing, focused = %v, query = %q; want the labelled field focused with Ada", h.tt.Focused("Search"), h.p.Query())
	}
}
