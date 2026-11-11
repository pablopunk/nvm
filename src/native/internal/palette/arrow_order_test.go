package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A host that replaces the query (a form leaving a field edit) leaves the search field's text behind until the next
// frame. An arrow that arrives in that gap moves the choice at once, in order with the Enter that follows it, and
// leaves the field's text for the frame to replace.
func TestArrowArrivingBeforeTheFieldCatchesUpWithAReplacedQueryStepsAtOnce(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Rows", &ran, "One", "Two", "Three", "Four"), true)
	h.p.SetQuery("")
	for want := 1; want <= 3; want++ {
		handled, syncField := h.p.arrowOnArrival(ui.KeyDown, "Ada Lovelace")
		if !handled || syncField || h.p.chosen != want {
			t.Fatalf("Down %d: handled %v, sync %v, chosen %d", want, handled, syncField, h.p.chosen)
		}
	}
}

func TestArrowOnArrivalSyncsTheFieldOnlyWhenItHoldsTheQuery(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Rows", &ran, "One", "Two"), true)
	if handled, syncField := h.p.arrowOnArrival(ui.KeyDown, ""); !handled || !syncField {
		t.Fatalf("handled %v, sync %v, want both", handled, syncField)
	}
}

// A search field that has been focused for a frame takes ArrowDown as the key arrives, so the palette claims no
// window shortcut for it: a claimed key is held for the next frame and would run after an Enter pressed behind it.
func TestFocusedSearchFieldTakesTheArrowsWithoutAFrameShortcut(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Rows", &ran, "One", "Two", "Three", "Four"), true)
	h.frame()
	if !h.p.searchFocused || !h.p.searchFocusedLast {
		t.Fatalf("the field should be focused for two frames: now %v, before %v", h.p.searchFocused, h.p.searchFocusedLast)
	}
	for want := 1; want <= 3; want++ {
		h.key(0, ui.KeyDown)
		if h.p.chosen != want {
			t.Fatalf("Down %d chose row %d", want, h.p.chosen)
		}
	}
	h.key(0, ui.KeyEnter)
	if len(ran) != 1 || ran[0] != "Four" {
		t.Fatalf("ran %v, want [Four]", ran)
	}
}
