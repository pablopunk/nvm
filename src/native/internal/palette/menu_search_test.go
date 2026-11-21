package palette

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

func TestActionSearchMatchesOneXRankingAndSections(t *testing.T) {
	acts := []search.Action{
		{Title: "Copy", Subtitle: "Open", Section: "First"},
		{Title: "Open", Section: "Second"},
		{Title: "Open file", Section: "Third"},
		{Title: "Open folder", Section: "Third"},
		{Title: "Reopen"},
		{Title: "Open hidden", MenuHidden: true},
	}
	ranked := rankedMenuActions(acts, "open")
	var titles []string
	for _, a := range ranked {
		titles = append(titles, a.Title)
	}
	if !slices.Equal(titles, []string{"Open", "Copy", "Open file", "Open folder", "Reopen"}) {
		t.Fatalf("ranking = %v", titles)
	}
	rows, _ := actionMenuRows(ranked, 0, "open")
	for _, row := range rows {
		if row.Header {
			t.Fatal("query kept section headers")
		}
	}
	if len(rankedMenuActions(acts, "oepn")) != 0 {
		t.Fatal("action menu used root search's character-bag typo fallback")
	}
	plain := rankedMenuActions(acts, "")
	if plain[0].Title != "Copy" || plain[1].Title != "Open" {
		t.Fatal("empty query changed source order")
	}
	rows, _ = actionMenuRows(plain, 0, "")
	if !rows[0].Header || rows[0].Title != "First" {
		t.Fatal("empty query lost sections")
	}
}

func TestMenuSearchRunsTheFilteredActionAndRestoresTheRootField(t *testing.T) {
	var ran []string
	h := newHarness(t, keepMenuView(twoActionView("Root", &ran, "Alpha")), true)
	h.typeText("al")
	h.key(Mod, ui.KeyK)
	h.typeText("secondary")
	if h.p.Query() != "al" || h.p.menuQuery != "secondary" || len(h.p.filteredMenuActions()) != 1 {
		t.Fatalf("queries root=%q menu=%q", h.p.Query(), h.p.menuQuery)
	}
	h.key(0, ui.KeyEnter)
	if !slices.Equal(ran, []string{"menu:Alpha"}) {
		t.Fatalf("filtered Enter ran %v", ran)
	}
	if h.p.actionsOpen {
		t.Fatal("menu remained open")
	}
	h.typeText("pha")
	if h.p.Query() != "alpha" {
		t.Fatalf("root field lost focus: %q", h.p.Query())
	}
}

func TestMenuSearchNoMatchesDoesNothingAndEscapeRestoresQuery(t *testing.T) {
	var ran []string
	h := newHarness(t, keepMenuView(twoActionView("Root", &ran, "Alpha")), true)
	h.typeText("al")
	h.key(Mod, ui.KeyK)
	h.typeText("zzzz")
	h.key(0, ui.KeyEnter)
	if len(ran) > 0 || !h.p.actionsOpen || !h.tt.HasText("No matching actions") {
		t.Fatal("no-match Enter ran an action or closed the menu")
	}
	h.key(0, ui.KeyEscape)
	h.typeText("pha")
	if h.p.Query() != "alpha" {
		t.Fatal("Escape did not restore the original input")
	}
	h.key(Mod, ui.KeyK)
	if h.p.menuQuery != "" || len(h.p.filteredMenuActions()) != 2 {
		t.Fatal("reopening kept the previous filter")
	}
}

func TestMenuKeepsWindowShortcutsAndClosesWhenItsOwnerDisappears(t *testing.T) {
	var ran []string
	h := newHarness(t, keepMenuView(hiddenMenuView(&ran)), true)
	h.key(Mod, ui.KeyK)
	h.typeText("copy")
	h.key(Mod, ui.KeyE)
	if !slices.Equal(ran, []string{"Clear"}) || h.p.actionsOpen {
		t.Fatalf("hidden window shortcut ran %v, menu open=%v", ran, h.p.actionsOpen)
	}
	h.key(Mod, ui.KeyK)
	h.p.showRows(h.p.cur(), h.p.Query(), []search.Item{{ID: "other", Title: "Other"}}, true, false)
	if h.p.actionsOpen {
		t.Fatal("menu survived replacement of its target")
	}
}

func keepMenuView(view *View) *View {
	searchRows := view.Search
	view.Search = func(query string) []search.Item {
		rows := searchRows(query)
		for i := range rows {
			for j := range rows[i].Actions {
				rows[i].Actions[j].Dismiss = search.DismissNever
			}
		}
		return rows
	}
	return view
}

func TestMenuSearchRestoresOwnedComposerAndSideFocus(t *testing.T) {
	var input ui.Handle
	draft := ""
	page := &View{Title: "Chat", Input: &input, Body: func(c *ui.Context, _ search.Host) { ui.TextInputBase(c, &draft).Bind(&input) }, Actions: func() []search.Action { return []search.Action{{Title: "First"}, {Title: "Secondary"}} }}
	h := newHarness(t, page, true)
	h.typeText("draft")
	h.key(Mod, ui.KeyK)
	h.typeText("secondary")
	if draft != "draft" || h.p.menuQuery != "secondary" {
		t.Fatalf("composer=%q menu=%q", draft, h.p.menuQuery)
	}
	h.key(0, ui.KeyEscape)
	h.typeText(" kept")
	if draft != "draft kept" {
		t.Fatalf("composer did not regain focus: %q", draft)
	}
	var ran []string
	side := NewSide(keepMenuView(twoActionView("Preview", &ran, "Alpha")))
	split := newSplitHarness(t, splitRoot(rowsView("Main", &ran, "Main"), side))
	split.typeText("main")
	side.Focus()
	split.frame()
	split.typeText("side")
	split.key(Mod, ui.KeyK)
	split.typeText("secondary")
	if side.shell.Query() != "side" || split.p.Query() != "main" || side.shell.menuQuery != "secondary" {
		t.Fatalf("menu typing changed a pane query: side=%q main=%q menu=%q open=%v focused=%v", side.shell.Query(), split.p.Query(), side.shell.menuQuery, side.shell.actionsOpen, side.Focused())
	}
	split.key(0, ui.KeyEscape)
	split.typeText("x")
	if !side.Focused() || side.shell.Query() != "sidex" {
		t.Fatal("closing side menu lost side focus")
	}
}

func TestMenuSearchAndEnterInOneFrameUseTheLatestQuery(t *testing.T) {
	var ran []string
	h := newHarness(t, keepMenuView(twoActionView("Root", &ran, "Alpha")), true)
	h.key(Mod, ui.KeyK)
	h.tt.Type("secondary")
	h.tt.Key(0, ui.KeyEnter)
	h.frame()
	if !slices.Equal(ran, []string{"menu:Alpha"}) {
		t.Fatalf("rapid search/Enter ran %v", ran)
	}
}
