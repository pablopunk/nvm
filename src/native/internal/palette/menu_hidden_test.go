package palette

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// hiddenMenuView is a list whose one row has a primary, a menu action and a shortcut-only action. Each run appends its
// title to ran.
func hiddenMenuView(ran *[]string) *View {
	return &View{
		Title:       "Hidden",
		Placeholder: "Search Hidden",
		Search: func(string) []search.Item {
			return []search.Item{{Kind: "Test", Title: "Alpha", Actions: []search.Action{
				{Title: "Open", Run: func(search.Host) { *ran = append(*ran, "Open") }},
				{Title: "Copy", Run: func(search.Host) { *ran = append(*ran, "Copy") }},
				{Title: "Clear", Label: "⌃E", Mods: Mod, Key: ui.KeyE, MenuHidden: true, Run: func(search.Host) { *ran = append(*ran, "Clear") }},
			}}}
		},
	}
}

// TestMenuHiddenActionIsNotListedButItsShortcutRuns checks an action marked MenuHidden is left out of the action menu,
// while its shortcut still runs it, as 1.x keeps local shortcuts when a panel is hidden.
func TestMenuHiddenActionIsNotListedButItsShortcutRuns(t *testing.T) {
	var ran []string
	h := newHarness(t, hiddenMenuView(&ran), true)
	h.key(Mod, ui.KeyK)
	if got := menuTitlesOf(h.p); !slices.Equal(got, []string{"Open", "Copy"}) {
		t.Fatalf("menu = %v, want the visible actions only", got)
	}

	h.key(0, ui.KeyEscape)
	h.key(Mod, ui.KeyE)
	if !slices.Equal(ran, []string{"Clear"}) {
		t.Fatalf("ran = %v after the shortcut, want the hidden action", ran)
	}
}

// TestEnterInTheMenuRunsTheChosenVisibleAction checks the menu's index refers to the visible actions, so the second
// visible row is Copy even though Clear sits between them in the row's actions.
func TestEnterInTheMenuRunsTheChosenVisibleAction(t *testing.T) {
	var ran []string
	h := newHarness(t, hiddenMenuView(&ran), true)
	h.key(0, ui.KeyEnter)
	h.key(Mod, ui.KeyK)
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyEnter)
	if !slices.Equal(ran, []string{"Open", "Copy"}) {
		t.Fatalf("ran = %v, want Enter on the row and then the second visible menu row", ran)
	}
}

// TestRowWhoseActionsAreAllHiddenHasNoMenu checks ⌘K does nothing when every action of the row is hidden from the menu,
// while Enter still runs the primary.
func TestRowWhoseActionsAreAllHiddenHasNoMenu(t *testing.T) {
	var ran []string
	view := &View{Title: "Hidden", Search: func(string) []search.Item {
		return []search.Item{{Kind: "Test", Title: "Alpha", Actions: []search.Action{
			{Title: "Open", MenuHidden: true, Run: func(search.Host) { ran = append(ran, "Open") }},
		}}}
	}}
	h := newHarness(t, view, true)
	h.key(Mod, ui.KeyK)
	if h.p.actionsOpen {
		t.Fatal("⌘K opened a menu whose every action is hidden")
	}
	h.key(0, ui.KeyEnter)
	if !slices.Equal(ran, []string{"Open"}) {
		t.Fatalf("ran = %v, want Enter to run the primary", ran)
	}
}

func menuTitlesOf(p *Palette) []string {
	var titles []string
	for _, action := range p.menuActions() {
		titles = append(titles, action.Title)
	}
	return titles
}

func TestMenuHidesDuplicateShortcutLabels(t *testing.T) {
	view := &View{Title: "Actions", Search: func(string) []search.Item {
		return []search.Item{{Title: "Item", Actions: []search.Action{{Title: "First", Label: "⌘F"}, {Title: "Second", Label: "⌘F"}}}}
	}}
	h := newHarness(t, view, true)
	actions := h.p.menuActions()
	if len(actions) != 2 || actions[0].Label != "⌘F" || actions[1].Label != "" {
		t.Fatalf("actions = %+v", actions)
	}
}
