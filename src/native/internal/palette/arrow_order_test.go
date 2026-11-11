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
