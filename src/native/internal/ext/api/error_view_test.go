package api

import "testing"

func TestErrorViewCarriesTheMessageAsContextualGuidance(t *testing.T) {
	view := ErrorView("Invalid date", "Use YYYY-MM-DD, for example 2026-11-04.")
	if view.Kind != ViewList || view.Title != "Invalid date" || len(view.Items) != 1 {
		t.Fatalf("view = %+v; want one list item under the title", view)
	}
	item := view.Items[0]
	if !item.Disabled || !item.Contextual || item.Subtitle != "Use YYYY-MM-DD, for example 2026-11-04." {
		t.Fatalf("item = %+v; want full-contrast contextual guidance", item)
	}
	if err := view.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
