package app

import "testing"

// There is one floating window, as in 1.x: showing another note in it replaces the note, moves the floating flag to
// it, and ends the session of the note it showed.
func TestShowingAnotherNoteInTheFloatingWindowReplacesTheNote(t *testing.T) {
	p := goldenPalette(t)
	first, _ := p.acquireEdit("note-ideas")
	second, _ := p.acquireEdit("note-release")
	p.floating = &floatingWindow{edit: first}
	_ = p.notes.SetFloating("note-ideas", true)

	p.showInFloatingWindow(second)

	if p.floating.edit != second {
		t.Fatal("the window does not show the note it was given")
	}
	if n, _ := p.notes.Get("note-ideas"); n.Floating {
		t.Fatal("the note the window left still floats")
	}
	if n, _ := p.notes.Get("note-release"); !n.Floating {
		t.Fatal("the note the window shows does not float")
	}
	if _, ok := p.edits["note-ideas"]; ok {
		t.Fatal("the note the window left keeps its session")
	}
}

// Deleting the note the floating window shows puts the most recent other note in the window, as 1.x does.
func TestDeletingTheFloatingNoteShowsTheMostRecentNote(t *testing.T) {
	p := goldenPalette(t)
	shown, _ := p.acquireEdit("note-ideas")
	p.floating = &floatingWindow{edit: shown}

	p.deleteNote("note-ideas")

	if p.floating == nil || p.floating.edit == nil || p.floating.edit.id == "note-ideas" {
		t.Fatal("the window does not show another note after the delete")
	}
	if p.floating.edit.id != p.notes.List()[0].ID {
		t.Fatalf("window shows %q, want the most recent note %q", p.floating.edit.id, p.notes.List()[0].ID)
	}
}
