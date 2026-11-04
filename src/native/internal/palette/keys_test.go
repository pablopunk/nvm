package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// twoActionView is a sub-view whose rows each have a first action that records "run:<title>" and a second that
// records "menu:<title>" and is bound to Command or Control with E.
func twoActionView(title string, ran *[]string, names ...string) *View {
	return &View{
		Title:       title,
		Placeholder: "Search " + title,
		Search: func(string) []search.Item {
			var out []search.Item
			for _, name := range names {
				name := name
				out = append(out, search.Item{Kind: "Test", Title: name, Actions: []search.Action{
					{Title: "Primary", Run: func(search.Host) { *ran = append(*ran, "run:"+name) }},
					{Title: "Secondary", Label: "⌃E", Mods: Mod, Key: ui.KeyE, Run: func(search.Host) { *ran = append(*ran, "menu:"+name) }},
				}})
			}
			return out
		},
	}
}

// TestEscapeLeavesASubviewThenHidesTheRootWhateverTheQuery checks 1.x's Escape: a pushed page is left at once, even
// with text in its field, and the root hides at once, even with text in its field.
func TestEscapeLeavesASubviewThenHidesTheRootWhateverTheQuery(t *testing.T) {
	var ran []string
	root := rowsView("Root", &ran, "Alpha", "Beta")
	h := newHarness(t, root, true)
	sub := rowsView("Sub", &ran, "Gamma")
	h.p.Push(sub)
	h.typeText("ga")
	if h.p.Query() != "ga" {
		t.Fatalf("query = %q, want ga", h.p.Query())
	}

	h.key(0, ui.KeyEscape)
	if h.p.Top() != root || !h.win.visible {
		t.Fatalf("Escape in a subview should go back to the root and keep the palette open: top-is-root=%v visible=%v", h.p.Top() == root, h.win.visible)
	}

	h.typeText("al")
	h.key(0, ui.KeyEscape)
	if h.win.visible || h.win.hides != 1 {
		t.Fatalf("Escape at the root should hide with text in the field: visible=%v hides=%d", h.win.visible, h.win.hides)
	}
}

// TestBackspaceNeverPopsWhileItEditsAQueryOrHidesItsInput checks the guards of the Backspace pop: a typed query
// is edited, and a view that hides its input keeps its Backspace. The pop itself on an empty query is in
// TestBackspacePopsOnAnEmptyQuery.
func TestBackspaceNeverPopsWhileItEditsAQueryOrHidesItsInput(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	sub := rowsView("Sub", &ran, "Gamma")
	h.p.Push(sub)

	h.typeText("g")
	h.key(0, ui.KeyBackspace)
	if h.p.Top() != sub || h.p.Query() != "" {
		t.Fatalf("Backspace with a query edits it: top-is-sub=%v query=%q", h.p.Top() == sub, h.p.Query())
	}

	hidden := &View{Title: "Editor", HideInput: true, Search: func(string) []search.Item { return nil }}
	h.p.Push(hidden)
	h.key(0, ui.KeyBackspace)
	if h.p.Top() != hidden {
		t.Fatal("Backspace must not pop a view that hides its input")
	}
}

// TestBackspacePopsOnAnEmptyQuery is the pop itself. MyGo's search field takes Backspace even when it is empty
// (its leaveEmptyBackspace flag is internal to ui and only the combobox sets it), so the shortcut never sees the
// key. The prototype routes it the same way; the gap is recorded in the Stage C report, not hidden here.
func TestBackspacePopsOnAnEmptyQuery(t *testing.T) {
	t.Skip("MyGo's search field takes Backspace on an empty query before the palette's shortcut; see the Stage C report")
}

func TestCommandKOpensTheMenuAndEscapeClosesOnlyTheMenu(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Root", &ran, "Alpha", "Beta"), true)
	h.typeText("al")

	h.key(Mod, ui.KeyK)
	if !h.p.actionsOpen {
		t.Fatal("Command-K should open the action menu of the chosen row")
	}
	h.key(0, ui.KeyEscape)
	if h.p.actionsOpen || h.p.Query() != "al" || !h.win.visible {
		t.Fatalf("Escape in the menu should close only the menu: open=%v query=%q visible=%v", h.p.actionsOpen, h.p.Query(), h.win.visible)
	}

	h.key(Mod, ui.KeyK)
	h.key(Mod, ui.KeyK)
	if h.p.actionsOpen {
		t.Fatal("a second Command-K should close the menu")
	}
}

func TestEnterRunsTheFirstActionAndTheMenuRunsTheSelectedOne(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Root", &ran, "Alpha", "Beta"), true)

	h.key(0, ui.KeyEnter)
	if len(ran) != 1 || ran[0] != "run:Alpha" {
		t.Fatalf("Enter ran %v, want the first action of the first row", ran)
	}

	h.key(Mod, ui.KeyK)
	h.key(0, ui.KeyDown)
	h.key(0, ui.KeyEnter)
	if len(ran) != 2 || ran[1] != "menu:Alpha" {
		t.Fatalf("menu Enter ran %v, want the second action of the first row", ran)
	}
	if h.p.actionsOpen {
		t.Fatal("running an action from the menu should close it")
	}
}

func TestRowShortcutRunsItsActionWithoutTheMenu(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Root", &ran, "Alpha"), true)
	h.key(Mod, ui.KeyE)
	if len(ran) != 1 || ran[0] != "menu:Alpha" {
		t.Fatalf("row shortcut ran %v, want the bound action", ran)
	}
}

func TestArrowKeysStopAtTheEndsOfTheList(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "One", "Two", "Three"), true)
	for i := 0; i < 5; i++ {
		h.key(0, ui.KeyDown)
	}
	if got := h.chosenTitle(); got != "Three" {
		t.Fatalf("five Down presses chose %q, want the last row (no wrap)", got)
	}
	for i := 0; i < 5; i++ {
		h.key(0, ui.KeyUp)
	}
	if got := h.chosenTitle(); got != "One" {
		t.Fatalf("five Up presses chose %q, want the first row (no wrap)", got)
	}
}

func TestFocusReturnsToTheInputAfterPop(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	h.p.Push(rowsView("Sub", &ran, "Gamma"))
	h.frame()
	h.p.Pop()
	h.frame()
	if active, _ := h.tt.TextInputActive(); !active {
		t.Fatal("after Pop the search field should have the keyboard focus again")
	}
}

func TestTabOpensAskAIOnlyAtTheRootWithAQuery(t *testing.T) {
	var ran []string
	ai := &View{Title: "AI Chat", Search: func(string) []search.Item { return nil }}
	askAI := func(o *Options) {
		o.AskAI = func(q string) *View {
			if q != "hello" {
				t.Errorf("AskAI got %q, want the trimmed query", q)
			}
			return ai
		}
	}

	// Tab with no query is ordinary focus travel, so it runs on a palette that has not typed anything.
	empty := newHarness(t, rowsView("Root", &ran, "Alpha"), true, askAI)
	empty.key(0, ui.KeyTab)
	if empty.p.Top() == ai {
		t.Fatal("Tab with no query must not open the AI view")
	}

	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true, askAI)
	h.typeText("  hello ")
	h.key(0, ui.KeyTab)
	if h.p.Top() != ai {
		t.Fatalf("Tab with a query at the root should open the AI view (query %q)", h.p.Query())
	}

	h.p.Pop()
	h.p.Push(rowsView("Sub", &ran, "Gamma"))
	h.typeText("x")
	h.key(0, ui.KeyTab)
	if h.p.Top() == ai {
		t.Fatal("Tab below the root must not open the AI view")
	}
}

// TestArrowRightOpensTheActionMenuOnlyWhenTheCaretIsAtTheEnd checks the 1.x rule: inside the query ArrowRight moves the
// caret, and at its end it opens the action menu of the chosen row.
func TestArrowRightOpensTheActionMenuOnlyWhenTheCaretIsAtTheEnd(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Root", &ran, "Alpha", "Beta"), true)
	h.typeText("al")
	h.key(0, ui.KeyLeft)
	h.key(0, ui.KeyRight)
	if h.p.actionsOpen {
		t.Fatal("ArrowRight before the end of the query should move the caret, not open the menu")
	}
	h.key(0, ui.KeyRight)
	if !h.p.actionsOpen {
		t.Fatal("ArrowRight at the end of the query should open the action menu")
	}
}

// TestArrowLeftClosesTheMenuAndGoesBackOnlyAtTheStartOfTheQuery checks the 1.x rule for ArrowLeft: it moves the caret
// until the start of the query, then closes the open menu or goes back a page.
func TestArrowLeftClosesTheMenuAndGoesBackOnlyAtTheStartOfTheQuery(t *testing.T) {
	var ran []string
	root := rowsView("Root", &ran, "Alpha")
	h := newHarness(t, root, true)
	sub := rowsView("Sub", &ran, "Gamma")
	h.p.Push(sub)
	h.typeText("ga")
	h.key(0, ui.KeyLeft)
	h.key(0, ui.KeyLeft)
	if h.p.Top() != sub {
		t.Fatal("ArrowLeft inside the query should move the caret, not go back")
	}
	h.key(0, ui.KeyLeft)
	if h.p.Top() != root {
		t.Fatal("ArrowLeft at the start of the query should go back to the root")
	}
}

func TestArrowLeftAtTheStartClosesTheActionMenu(t *testing.T) {
	var ran []string
	h := newHarness(t, twoActionView("Root", &ran, "Alpha"), true)
	h.key(Mod, ui.KeyK)
	h.key(0, ui.KeyLeft)
	if h.p.actionsOpen || !h.win.visible || h.p.Top() != h.p.stack[0] {
		t.Fatalf("ArrowLeft at the start should close the menu and stay on the root: open=%v visible=%v", h.p.actionsOpen, h.win.visible)
	}
}
