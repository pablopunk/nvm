package extview

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// categoryGrid is a grid that declares CategoryKeys over three categories. Each change is recorded with the value it
// was run with.
func categoryGrid(changes *[]string) *Session {
	view := api.View{
		Kind: api.ViewGrid, Title: "Symbols",
		Grid: &api.GridSpec{CategoryKeys: true},
		Filter: &api.ViewFilter{
			ID: "category", Value: "all",
			Options:  []api.ViewFilterOption{{Title: "All", Value: "all"}, {Title: "Music", Value: "music"}, {Title: "Arrows", Value: "arrows"}},
			OnChange: &api.ActionRef{ID: "category"},
		},
	}
	deps := Deps{
		Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			*changes = append(*changes, in.Value)
			done(api.Result{}, nil)
		},
		OnUI:  func(fn func()) { fn() },
		Clock: fake.NewClock(),
	}
	s := NewSession(deps, "acme.categories", view)
	s.Open(&fakeHost{})
	return s
}

// TestTabAndShiftTabStepTheCategoriesAndWrap steps the categories forward and back, past both ends, and checks each
// value the change handler runs with and the value the accessory shows.
func TestTabAndShiftTabStepTheCategoriesAndWrap(t *testing.T) {
	var changes []string
	s := categoryGrid(&changes)
	s.stepCategory(1)
	s.stepCategory(1)
	s.stepCategory(1)
	s.stepCategory(-1)
	if want := []string{"music", "arrows", "all", "arrows"}; !slices.Equal(changes, want) {
		t.Fatalf("change handler ran with %v, want %v", changes, want)
	}
	if s.accessory.Value != "arrows" {
		t.Fatalf("accessory shows %q, want arrows", s.accessory.Value)
	}
}

// TestGridWithoutCategoryKeysDoesNotStepOnTab keeps the keys off unless the grid declares them.
func TestGridWithoutCategoryKeysDoesNotStepOnTab(t *testing.T) {
	var changes []string
	s := categoryGrid(&changes)
	s.view.Grid.CategoryKeys = false
	if s.Palette().OnKey != nil {
		t.Fatal("a grid without CategoryKeys must not take Tab")
	}
}

// TestGridWithCategoryKeysDrawsItsCategoryAccessory keeps the category picker in the search field, as the 1.x picker
// shows it, while Tab steps through the same options.
func TestGridWithCategoryKeysDrawsItsCategoryAccessory(t *testing.T) {
	var changes []string
	s := categoryGrid(&changes)
	accessory := s.gridPage().Accessory
	if accessory == nil || accessory.Value != "all" || len(accessory.Options) != 3 {
		t.Fatalf("a grid with CategoryKeys must draw its accessory showing all, got %+v", accessory)
	}
}
