package extview

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// menuRig is a session over view whose invoke port is never needed by the menu tests.
func menuRig(view api.View) *Session {
	return NewSession(Deps{Files: fake.NewFS()}, "acme.menu", view)
}

// menuTitles returns the titles of the actions, and which of them the menu hides.
func menuTitles(actions []search.Action) ([]string, []bool) {
	titles := make([]string, 0, len(actions))
	hidden := make([]bool, 0, len(actions))
	for _, action := range actions {
		titles = append(titles, action.Title)
		hidden = append(hidden, action.MenuHidden)
	}
	return titles, hidden
}

func panelOf(titles ...string) *api.ActionPanel {
	actions := make([]api.Action, 0, len(titles))
	for _, title := range titles {
		actions = append(actions, api.Run(title, title, nil))
	}
	return api.Panel(api.Section("", actions...))
}

// TestItemMenuListsThePrimaryThenItsPanel checks an item's menu follows 1.x: the primary first, then the action panel,
// and an action the primary repeats is listed once.
func TestItemMenuListsThePrimaryThenItsPanel(t *testing.T) {
	primary := api.Run("open", "Open", nil)
	item := api.Item{ID: "note", Title: "Note", Primary: &primary, ActionPanel: api.Panel(api.Section("", api.Run("open", "Open", nil), api.Run("copy", "Copy", nil)))}
	row := menuRig(api.ListView("Notes", item)).row(item)

	titles, _ := menuTitles(row.Actions)
	if !slices.Equal(titles, []string{"Open", "Copy"}) {
		t.Fatalf("menu = %v, want the primary then the panel without its repeat", titles)
	}
}

// TestHiddenItemPanelKeepsEnterButNoMenu checks a hidden item panel keeps every action out of the menu, the primary
// included, so the menu cannot open, while the item's actions stay on the row for Enter and their shortcuts.
func TestHiddenItemPanelKeepsEnterButNoMenu(t *testing.T) {
	primary := api.Run("open", "Open", nil)
	item := api.Item{
		ID: "note", Title: "Note", Primary: &primary, ActionPanel: panelOf("Copy"),
		ActionPanelVisibility: api.PanelHidden,
	}
	row := menuRig(api.ListView("Notes", item)).row(item)

	titles, hidden := menuTitles(row.Actions)
	if !slices.Equal(titles, []string{"Open", "Copy"}) || !slices.Equal(hidden, []bool{true, true}) {
		t.Fatalf("actions = %v hidden = %v, want both kept for Enter and shortcuts and both hidden from the menu", titles, hidden)
	}
}

// TestHiddenViewPanelLeavesItsActionsOutOfTheItemMenu checks a view's actions join an item's menu only when the view
// shows its panel, and a hidden view panel keeps them as shortcut-only actions.
func TestHiddenViewPanelLeavesItsActionsOutOfTheItemMenu(t *testing.T) {
	item := api.Item{ID: "note", Title: "Note", Primary: ptrAction(api.Run("open", "Open", nil)), Actions: []api.Action{api.Run("rename", "Rename", nil)}}
	view := api.ListView("Notes", item)
	view.ActionPanel = panelOf("Clear all")
	view.ActionPanelVisibility = api.PanelHidden

	titles, hidden := menuTitles(menuRig(view).row(item).Actions)
	if !slices.Equal(titles, []string{"Open", "Rename", "Clear all"}) || !slices.Equal(hidden, []bool{false, false, true}) {
		t.Fatalf("menu = %v hidden = %v, want the view action kept but hidden from the menu", titles, hidden)
	}
}

// TestRowWithoutActionsListsTheViewActionsButRunsNone checks a row with no action of its own still lists the view's
// actions in its menu, as 1.x does, and Enter runs none of them.
func TestRowWithoutActionsListsTheViewActionsButRunsNone(t *testing.T) {
	item := api.Item{ID: "plain", Title: "Plain"}
	view := api.ListView("Notes", item)
	view.ActionPanel = panelOf("Clear all")

	row := menuRig(view).row(item)
	titles, _ := menuTitles(row.Actions)
	if !slices.Equal(titles, []string{"Clear all"}) || !row.Actions[0].MenuOnly {
		t.Fatalf("menu = %v menu-only = %v; want the view action listed and held back from Enter", titles, row.Actions[0].MenuOnly)
	}
	if _, ok := row.DefaultAction(); ok {
		t.Fatal("a row with only view actions has a default action; Enter would run a view action")
	}
}

// TestMenuGroupsTitleThePanelSectionsAndTheViewActions checks each action carries the title of its group: the panel's
// section titles, and View for the view's actions an item does not repeat.
func TestMenuGroupsTitleThePanelSectionsAndTheViewActions(t *testing.T) {
	primary := api.Run("open", "Open", nil)
	item := api.Item{ID: "note", Title: "Note", Primary: &primary, ActionPanel: api.Panel(
		api.Section("Manage", api.Run("pin", "Pin", nil)), api.Section("", api.Run("copy", "Copy", nil)))}
	view := api.ListView("Notes", item)
	view.ActionPanel = panelOf("Clear all")

	var groups []string
	for _, action := range menuRig(view).row(item).Actions {
		groups = append(groups, action.Section)
	}
	if !slices.Equal(groups, []string{"", "Manage", "", "View"}) {
		t.Fatalf("groups = %q, want the primary, the Manage section, the untitled section and View", groups)
	}
}

// TestPageMenuListsTheViewPanelWhenNoRowIsChosen checks an empty list offers the view's own panel in its menu, and a
// hidden view panel offers none.
func TestPageMenuListsTheViewPanelWhenNoRowIsChosen(t *testing.T) {
	view := api.ListView("Notes")
	view.ActionPanel = panelOf("Clear all", "Export")
	titles, hidden := menuTitles(menuRig(view).pageActions())
	if !slices.Equal(titles, []string{"Clear all", "Export"}) || slices.Contains(hidden, true) {
		t.Fatalf("page menu = %v hidden = %v, want both view actions visible", titles, hidden)
	}

	view.ActionPanelVisibility = api.PanelHidden
	_, hidden = menuTitles(menuRig(view).pageActions())
	if !slices.Equal(hidden, []bool{true, true}) {
		t.Fatalf("hidden page menu flags = %v, want both hidden", hidden)
	}
}
