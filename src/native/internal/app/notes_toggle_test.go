package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// The Toggle Floating Notes command is searchable but not suggested at the empty root, so the root list stays as it was.
func TestToggleFloatingNotesIsSearchedButNotSuggested(t *testing.T) {
	p := goldenPalette(t)
	if hasRowTitled(commandRows(p, &rootInput{}, ""), "Toggle Floating Notes") {
		t.Fatal("the empty root suggests Toggle Floating Notes")
	}
	if !hasRowTitled(commandRows(p, &rootInput{}, "toggle floating"), "Toggle Floating Notes") {
		t.Fatal("searching for the command does not find it")
	}
}

// Toggle with no floating note open picks the note saved last, and creates one when no note exists.
func TestMostRecentNoteIDPicksTheLatestAndCreatesWhenEmpty(t *testing.T) {
	p := goldenPalette(t)
	if p.floating != nil {
		t.Fatal("a fresh palette has floating notes open")
	}
	latest := p.notes.List()[0]
	if id, err := p.mostRecentNoteID(); err != nil || id != latest.ID {
		t.Fatalf("most recent = %q (%v), want %q", id, err, latest.ID)
	}
	for _, n := range p.notes.List() {
		if err := p.notes.Delete(n.ID); err != nil {
			t.Fatal(err)
		}
	}
	id, err := p.mostRecentNoteID()
	if err != nil || id == "" || len(p.notes.List()) != 1 {
		t.Fatalf("empty store: id %q, err %v, notes %d", id, err, len(p.notes.List()))
	}
}

// Command+O in the editor replaces the editor with All notes, as 1.x's note menu does.
func TestEditorCommandOOpensAllNotesInPlaceOfTheEditor(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.notesView())
	view, _ := p.noteEditorView("note-ideas")
	p.Push(view)
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(mod, ui.KeyO)
	tt.Frame()
	if top := topView(p); top.Title != "Quick Notes" {
		t.Fatalf("top view = %q, want All notes", top.Title)
	}
	p.Pop()
	if top := topView(p); top.Title != "Quick Notes" {
		t.Fatalf("after one Escape the top view is %q, want the list the editor was opened from", top.Title)
	}
	p.Pop()
	if top := topView(p); top.Title != rootViewTitle {
		t.Fatalf("after two Escapes the top view is %q, want the root", top.Title)
	}
}

// Delete asks with the 1.x question, and the button says Delete note.
func TestDeleteConfirmationUses1xCopy(t *testing.T) {
	p := goldenPalette(t)
	p.Push(p.confirmDeleteNote("note-ideas", "Ideas"))
	top := p.Top()
	if top.Placeholder != "Delete note?" {
		t.Fatalf("placeholder = %q", top.Placeholder)
	}
	if row := p.Results()[0]; row.Title != "Delete note" || row.Subtitle != "Delete “Ideas”? This cannot be undone." {
		t.Fatalf("confirmation row = %q / %q", row.Title, row.Subtitle)
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
