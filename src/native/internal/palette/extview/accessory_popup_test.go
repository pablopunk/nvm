package extview

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// notesWithFilter is the list view of the acme.views fixture's notes, with its search accessory: a view-level dropdown
// the extension filters by.
var notesWithFilter = api.View{
	Kind: api.ViewList, Title: "Notes",
	Items: []api.Item{{ID: "n1", Title: "Draft plan"}, {ID: "n2", Title: "Release plan", Subtitle: "Done"}},
	Filter: &api.ViewFilter{
		Value:    "all",
		Options:  []api.ViewFilterOption{{Title: "All", Value: "all"}, {Title: "Done", Value: "done"}},
		OnChange: &api.ActionRef{ID: "filter"},
	},
}

// TestViewFilterOpensANativeMenuAndChoosesItsOption shows an extension's search accessory in a real palette. A
// press opens the native menu of its options, the current one checked, and choosing one makes it the value and runs
// the extension's handler.
func TestViewFilterOpensANativeMenuAndChoosesItsOption(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{"filter": {}}}
	session := NewSession(log.deps(fake.NewFS()), "acme.views", notesWithFilter)
	page := session.listPage()
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()

	if err := tt.Click("All"); err != nil {
		t.Fatalf("press the accessory: %v", err)
	}
	tt.Frame()
	if got := tt.Menu(); !slices.Equal(got, []string{"All", "Done"}) {
		t.Fatalf("menu = %v, want the accessory's options All and Done", got)
	}
	if err := tt.ChooseMenuItem("Done"); err != nil {
		t.Fatalf("choose Done: %v", err)
	}
	tt.Frame()
	if page.Accessory.Value != "done" {
		t.Fatalf("accessory value = %q after choosing Done, want done", page.Accessory.Value)
	}
	if !slices.Equal(log.calls, []string{"acme.views:filter"}) {
		t.Fatalf("handler calls = %v, want the filter handler once", log.calls)
	}
}

// TestViewFilterClosedWithoutAChoiceChangesNothing checks Escape or a click elsewhere leaves the value and the
// handler alone.
func TestViewFilterClosedWithoutAChoiceChangesNothing(t *testing.T) {
	log := &invokeLog{results: map[string]api.Result{"filter": {}}}
	session := NewSession(log.deps(fake.NewFS()), "acme.views", notesWithFilter)
	page := session.listPage()
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	p.Show()
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()

	if err := tt.Click("All"); err != nil {
		t.Fatalf("press the accessory: %v", err)
	}
	tt.Frame()
	tt.CloseMenu()
	tt.Frame()
	if page.Accessory.Value != "all" || len(log.calls) != 0 {
		t.Fatalf("value = %q, calls = %v after closing the menu; want all and no handler call", page.Accessory.Value, log.calls)
	}
}

// TestAccessoryStepsFromTheKeyboardAndAnswersInPlace checks that a view with a filter lists Next and Previous actions
// without the extension asking for them, binds no key to them (a bracket is not on the same place of every keyboard
// layout), and that a handler answering with a replacing view changes the open page's rows.
func TestAccessoryStepsFromTheKeyboardAndAnswersInPlace(t *testing.T) {
	replacement := api.View{Kind: api.ViewList, Title: "Notes", Items: []api.Item{{ID: "n2", Title: "Release plan"}},
		Filter: &api.ViewFilter{Value: "done", Options: notesWithFilter.Filter.Options, OnChange: &api.ActionRef{ID: "filter"}}}
	log := &invokeLog{results: map[string]api.Result{"filter": {View: &replacement, Navigation: api.NavReplace}}}
	session := NewSession(log.deps(fake.NewFS()), "acme.views", notesWithFilter)
	page := session.listPage()
	actions := session.accessoryActions()
	if len(actions) != 2 || actions[0].Title != "Next Filter" || actions[1].Title != "Previous Filter" {
		t.Fatalf("page actions = %+v, want Next Filter and Previous Filter", actions)
	}
	for _, action := range actions {
		if action.Dismiss != search.DismissNever {
			t.Fatalf("%q dismisses the palette", action.Title)
		}
		if action.Key != 0 || action.Mods != 0 {
			t.Fatalf("%q binds a key (%v %v); a filter action must bind none", action.Title, action.Mods, action.Key)
		}
	}
	actions[0].Run(nil)
	if page.Accessory.Value != "done" {
		t.Fatalf("accessory value = %q, want done", page.Accessory.Value)
	}
	if len(session.items) != 1 || session.items[0].ID != "n2" {
		t.Fatalf("items = %+v, want the replacement's rows", session.items)
	}
}
