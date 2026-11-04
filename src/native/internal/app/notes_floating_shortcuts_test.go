package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A window shortcut, pressed on the real editor, runs its action and types nothing into either note: a Command key
// types no text on a keyboard, so the editor gets only the key.
func TestWindowShortcutsDoNotTypeTheirLetterIntoTheNote(t *testing.T) {
	cases := []struct {
		name string
		key  ui.Key
	}{
		{"new note", ui.KeyN},
		{"all notes", ui.KeyO},
		{"actions menu", ui.KeyK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := goldenPalette(t)
			n, err := p.notes.Create(goldenNow)
			if err != nil {
				t.Fatal(err)
			}
			tt, _ := openFloatingFixture(t, p, n.ID)
			typeKeys(tt, "keep")
			tt.Frame()
			left := p.floating.edit

			tt.Key(mod, tc.key)
			tt.Frame()

			if got := left.sess.Text(); got != "keep" {
				t.Fatalf("the note left reads %q, want %q", got, "keep")
			}
			if p.floating != nil && p.floating.edit != left && p.floating.edit.sess.Text() != "" {
				t.Fatalf("the note shown reads %q, want an empty note", p.floating.edit.sess.Text())
			}
		})
	}
}

// Command+K opens the actions menu; the arrows choose a row, Enter runs it, and the menu is gone afterwards.
func TestCommandKOpensTheActionsMenuAndEnterRunsTheChosenRow(t *testing.T) {
	p := goldenPalette(t)
	tt, _ := openFloatingFixture(t, p, "note-ideas")

	tt.Key(mod, ui.KeyK)
	tt.Frame()
	if !p.floatPane.actionsOpen || !tt.HasText("Back") {
		t.Fatal("Command+K does not open the actions menu with the Back hint")
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if p.floatPane.actionsOpen || p.floatPane.pane != paneAllNotes {
		t.Fatalf("Enter on the second row: menu open %v, pane %d; want the All notes list", p.floatPane.actionsOpen, p.floatPane.pane)
	}
}

// Escape closes the actions menu first, and the window stays open.
func TestEscapeClosesTheActionsMenuBeforeTheWindow(t *testing.T) {
	p := goldenPalette(t)
	tt, surface := openFloatingFixture(t, p, "note-ideas")

	tt.Key(mod, ui.KeyK)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	if p.floatPane.actionsOpen || surface.closed {
		t.Fatalf("Escape with the menu open: menu %v, window closed %v", p.floatPane.actionsOpen, surface.closed)
	}
}
