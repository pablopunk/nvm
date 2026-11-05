package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// The Toggle Floating Notes command is searchable but not suggested at the empty root, so the root list stays as it was.
func TestToggleFloatingNotesIsSearchedButNotSuggested(t *testing.T) {
	p := rootTableFixture(t)
	if hasRowTitled(rootResultsForTable(p, ""), "Toggle Floating Notes") {
		t.Fatal("the empty root suggests Toggle Floating Notes")
	}
	if !hasRowTitled(rootResultsForTable(p, "toggle floating"), "Toggle Floating Notes") {
		t.Fatal("searching for the command does not find it")
	}
}

// Command+O in the editor replaces the editor with All notes, as 1.x's note menu does, and Escape returns to the root.
func TestEditorCommandOOpensAllNotesInPlaceOfTheEditor(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenNoteEditorPage(t, p))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(mod, ui.KeyO)
	p.settleExtensionCalls()
	tt.Frame()
	if top := topView(p); top.Title != "Floating Notes" {
		t.Fatalf("top view = %q, want All notes", top.Title)
	}
	p.Pop()
	if top := topView(p); top.Title != rootViewTitle {
		t.Fatalf("after one Escape the top view is %q, want the root", top.Title)
	}
}

func hasRowTitled(rows []search.Item, title string) bool {
	for _, it := range rows {
		if it.Title == title {
			return true
		}
	}
	return false
}
