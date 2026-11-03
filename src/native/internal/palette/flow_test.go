package palette

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// TestKeyboardOnlySession is the keyboard part of J27 that the shell can show without a window: find a command
// by typing, open it with Enter, go back with Escape, hide with Escape, and summon the palette again with its
// query cleared. At each step the search field keeps the keyboard, so focus is never stranded.
func TestKeyboardOnlySession(t *testing.T) {
	var ran []string
	clipboard := rowsView("Clipboard History", &ran, "Yesterday's link", "Shell command")
	root := &View{
		Title:       "Nevermind",
		Placeholder: "Watcha gonna do?",
		Search: func(q string) []search.Item {
			out := []search.Item{}
			for _, title := range []string{"Clipboard History", "Quit Nevermind"} {
				if strings.HasPrefix(strings.ToLower(title), q) {
					out = append(out, search.Item{Kind: search.KindCommands, Title: title, Actions: []search.Action{
						{Title: "Open", Run: func(h search.Host) { h.Push(clipboard) }},
					}})
				}
			}
			return out
		},
	}
	h := newHarness(t, root, true)
	assertFocused(t, h, "shown")

	h.typeText("cl")
	if got := h.titles(); len(got) != 1 || got[0] != "Clipboard History" {
		t.Fatalf("typing cl should leave the clipboard command, got %v", got)
	}
	assertFocused(t, h, "typed")

	h.key(0, ui.KeyEnter)
	if h.p.Top() != clipboard {
		t.Fatal("Enter on the command should open its view")
	}
	assertFocused(t, h, "opened")

	h.key(0, ui.KeyEscape)
	if h.p.Top() != root || !h.win.visible {
		t.Fatal("Escape with a query should clear the query and stay on the root")
	}
	h.key(0, ui.KeyEscape)
	if h.win.visible {
		t.Fatal("Escape at the root with no query should hide the palette")
	}

	h.p.Show()
	h.frame()
	if h.p.Query() != "" || !h.win.visible {
		t.Fatalf("summoning the palette again should reset the query: query=%q visible=%v", h.p.Query(), h.win.visible)
	}
	assertFocused(t, h, "summoned")
}

// assertFocused fails the test when the search field lost the keyboard focus while the palette is shown.
func assertFocused(t *testing.T, h *harness, step string) {
	t.Helper()
	if active, _ := h.tt.TextInputActive(); !active {
		t.Fatalf("%s: the search field lost the keyboard focus", step)
	}
}
