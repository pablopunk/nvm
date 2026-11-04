package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// fakeFloatingSurface stands in for the MyGo window: it records what the app asked of it.
type fakeFloatingSurface struct {
	focused, closed bool
	x, y, w, h      int
}

func (f *fakeFloatingSurface) Focus() {
	f.focused = true
}

func (f *fakeFloatingSurface) Close() {
	f.closed = true
}

func (f *fakeFloatingSurface) Position() (int, int) {
	return f.x, f.y
}

func (f *fakeFloatingSurface) Size() (int, int) {
	return f.w, f.h
}

// openFloatingFixture shows the note of id in a fake floating window and returns the window's test renderer.
func openFloatingFixture(t *testing.T, p *App, id string) (*ui.Tester, *fakeFloatingSurface) {
	t.Helper()
	edit, ok := p.acquireEdit(id)
	if !ok {
		t.Fatalf("note %q does not open", id)
	}
	surface := &fakeFloatingSurface{w: 640, h: 460}
	p.floating = &floatingWindow{window: surface, edit: edit}
	tt := ui.NewTester(func(c *ui.Context) { floatingNoteWindow(c, p, p.floating.edit) }, 640, 460)
	tt.Frame()
	return tt, surface
}

// Command+N in the window makes a note and shows it there, and the window stays one window.
func TestCommandNInTheFloatingWindowShowsANewNote(t *testing.T) {
	p := goldenPalette(t)
	tt, surface := openFloatingFixture(t, p, "note-ideas")
	before := len(p.notes.List())

	tt.Key(mod, ui.KeyN)
	tt.Frame()

	if got := len(p.notes.List()); got != before+1 {
		t.Fatalf("notes = %d, want %d", got, before+1)
	}
	if p.floating.edit.id == "note-ideas" || p.floating.window != surface {
		t.Fatalf("window shows %q, want the new note in the same window", p.floating.edit.id)
	}
}

// Command+O lists the notes in the window; Escape clears a search first and then closes the window.
func TestEscapeBacksOutOfTheListBeforeClosingTheWindow(t *testing.T) {
	p := goldenPalette(t)
	tt, surface := openFloatingFixture(t, p, "note-ideas")

	tt.Key(mod, ui.KeyO)
	tt.Frame()
	if p.floatPane.pane != paneAllNotes {
		t.Fatal("Command+O does not list the notes")
	}
	p.floatPane.query = "release"
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if p.floatPane.query != "" || surface.closed {
		t.Fatal("the first Escape does not clear the search alone")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !surface.closed {
		t.Fatal("the second Escape does not close the window")
	}
}

// Delete note asks first: Cancel keeps the note and returns to the editor.
func TestDeleteNoteInTheFloatingWindowAsksFirst(t *testing.T) {
	p := goldenPalette(t)
	tt, _ := openFloatingFixture(t, p, "note-ideas")

	if err := tt.Click("Delete note"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if p.floatPane.pane != paneConfirmDelete {
		t.Fatalf("pane = %d, want the confirmation", p.floatPane.pane)
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if p.floatPane.pane != paneEditor {
		t.Fatal("Cancel does not return to the editor")
	}
	if _, ok := p.notes.Get("note-ideas"); !ok {
		t.Fatal("Cancel deleted the note")
	}
}

// A conflict replaces the editor with its two choices, and a choice applies and shows the editor again.
func TestConflictChoicesReplaceTheEditorUntilOneIsChosen(t *testing.T) {
	p := goldenPaletteWith(t, true)
	tt, _ := openFloatingFixture(t, p, "note-ideas")

	if !tt.HasText("Keep saved version") || !tt.HasText("Restore unsaved changes") {
		t.Fatal("the conflict does not show its choices")
	}
	if err := tt.Click("Keep saved version"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, pending := p.floating.edit.sess.Conflict(); pending {
		t.Fatal("the choice did not resolve the conflict")
	}
	if tt.HasText("Keep saved version") {
		t.Fatal("the choices stay on screen after one is chosen")
	}
}
