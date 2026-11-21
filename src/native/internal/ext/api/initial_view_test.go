package api

import "testing"

func TestInitialViewOnlyAllowsEmptyVisibleListOrGrid(t *testing.T) {
	grid := GridView("Images", 3)
	if err := (Command{ID: "images", Title: "Images", InitialView: &grid}).Validate(); err != nil {
		t.Fatal(err)
	}
	grid.Items = []Item{{ID: "x", Title: "x"}}
	if err := (Command{ID: "images", Title: "Images", InitialView: &grid}).Validate(); err == nil {
		t.Fatal("initial rows accepted")
	}
	grid.Items = nil
	if err := (Command{ID: "images", Title: "Images", Mode: ModeNoView, InitialView: &grid}).Validate(); err == nil {
		t.Fatal("noView command accepted loading view")
	}
}
