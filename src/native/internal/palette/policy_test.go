package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestDismissalPolicyDecidesWhetherTheActionClosesThePalette checks the three policies: auto closes unless the
// action opened a view, never keeps the palette open, and always closes even after a push.
func TestDismissalPolicyDecidesWhetherTheActionClosesThePalette(t *testing.T) {
	var ran []string
	next := rowsView("Next", &ran, "Inner")
	root := &View{Title: "Root", Search: func(string) []search.Item {
		return []search.Item{
			{Kind: "Test", Title: "Auto", Actions: []search.Action{{Title: "Run", Run: func(search.Host) {}}}},
			{Kind: "Test", Title: "Never", Actions: []search.Action{{Title: "Run", Dismiss: search.DismissNever, Run: func(search.Host) {}}}},
			{Kind: "Test", Title: "Pushes", Actions: []search.Action{{Title: "Run", Run: func(h search.Host) { h.Push(next) }}}},
			{Kind: "Test", Title: "Always", Actions: []search.Action{{Title: "Run", Dismiss: search.DismissAlways, Run: func(h search.Host) { h.Push(next) }}}},
		}
	}}
	h := newHarness(t, root, true)

	h.p.runAction(h.p.Results()[0].Actions[0])
	if h.win.visible {
		t.Fatal("an auto action that opened nothing should hide the palette")
	}

	h.win.Show()
	h.p.runAction(h.p.Results()[1].Actions[0])
	if !h.win.visible {
		t.Fatal("a never action should keep the palette open")
	}

	h.win.Show()
	h.p.runAction(h.p.Results()[2].Actions[0])
	if !h.win.visible || h.p.Top() != next {
		t.Fatalf("an auto action that pushed a view should open it and keep the palette open: visible=%v", h.win.visible)
	}

	h.p.Pop()
	h.win.Show()
	h.p.runAction(h.p.Results()[3].Actions[0])
	if h.win.visible || h.p.Top() != next {
		t.Fatal("an always action should hide the palette even though it pushed a view")
	}
}

// TestConfirmRunsOnlyWhenItsActionIsChosen checks the confirmation view: Enter on the action runs it and hides;
// Cancel returns to where the confirmation was opened without running anything.
func TestConfirmRunsOnlyWhenItsActionIsChosen(t *testing.T) {
	ran := 0
	h := newHarness(t, &View{Title: "Root", Search: func(string) []search.Item { return nil }}, true)
	root := h.p.Top()

	h.p.Push(Confirm("Empty Trash", "Permanently delete trashed files", func() { ran++ }))
	h.frame()
	if h.p.Top().Placeholder != "Empty Trash?" {
		t.Fatalf("the confirmation should ask about the action, placeholder %q", h.p.Top().Placeholder)
	}
	h.key(0, ui.KeyEnter)
	if ran != 1 || h.win.visible {
		t.Fatalf("Enter on the confirmation should run the action once and hide: ran=%d visible=%v", ran, h.win.visible)
	}

	h.win.Show()
	h.p.Pop()
	h.p.Push(Confirm("Restart", "Reboot this computer", func() { ran++ }))
	h.frame()
	h.p.Choose(1)
	h.key(0, ui.KeyEnter)
	if ran != 1 || h.p.Top() != root {
		t.Fatalf("Cancel should return to the root without running: ran=%d top-is-root=%v", ran, h.p.Top() == root)
	}
}
