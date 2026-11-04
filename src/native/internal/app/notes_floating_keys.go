package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/notes"
)

// floatingKeys runs the window's shortcuts before its content takes the keys, as 1.x's window does: Command+N makes a
// note, Command+O lists the notes, Command+Backspace deletes the note shown or the one selected in the list, Escape
// backs out of a pane and then closes the window, and the arrows and Enter move through the list.
func (p *App) floatingKeys(c *ui.Context) {
	switch {
	case c.Shortcut(mod, ui.KeyN):
		p.createFloatingNote()
	case c.Shortcut(mod, ui.KeyO):
		p.showAllNotesInFloatingWindow()
	case c.Shortcut(mod, ui.KeyBackspace):
		p.floatingBackspace()
	case c.Shortcut(0, ui.KeyEscape):
		p.escapeFloatingWindow()
	case p.floatPane.pane == paneAllNotes:
		p.floatingListKeys(c)
	}
}

// floatingListKeys moves the selection through the list, row 0 being New note, and activates the selected row.
func (p *App) floatingListKeys(c *ui.Context) {
	rows := len(p.floatingListNotes()) + 1
	switch {
	case c.Shortcut(0, ui.KeyDown):
		p.floatPane.selected = min(p.floatPane.selected+1, rows-1)
	case c.Shortcut(0, ui.KeyUp):
		p.floatPane.selected = max(p.floatPane.selected-1, 0)
	case c.Shortcut(0, ui.KeyEnter):
		p.activateFloatingListRow()
	}
}

// activateFloatingListRow runs the selected row: New note makes one, a note shows it in the window.
func (p *App) activateFloatingListRow() {
	if p.floatPane.selected == 0 {
		p.createFloatingNote()
		return
	}
	if items := p.floatingListNotes(); p.floatPane.selected-1 < len(items) {
		p.showFloatingNoteFromList(items[p.floatPane.selected-1].ID)
	}
}

// floatingBackspace asks to delete the note the window shows, or deletes the note selected in the list, as the 1.x
// list does without asking.
func (p *App) floatingBackspace() {
	switch p.floatPane.pane {
	case paneEditor:
		p.askToDeleteFloatingNote()
	case paneAllNotes:
		if items := p.floatingListNotes(); p.floatPane.selected > 0 && p.floatPane.selected-1 < len(items) {
			p.deleteNote(items[p.floatPane.selected-1].ID)
		}
	}
}

// escapeFloatingWindow backs out of the confirmation or a list search first, then closes the window, as 1.x does.
func (p *App) escapeFloatingWindow() {
	switch {
	case p.floatPane.pane == paneConfirmDelete:
		p.floatPane.pane = paneEditor
	case p.floatPane.pane == paneAllNotes && p.floatPane.query != "":
		p.floatPane.query = ""
		p.floatPane.selected = 0
	case p.floating != nil:
		p.floating.window.Close()
	}
}

// createFloatingNote makes a note and shows it in the floating window, with Command+N and the New note rows.
func (p *App) createFloatingNote() {
	p.floatPane = floatingPaneState{}
	p.newFloatingNote()
}

// showAllNotesInFloatingWindow shows the list of notes in the window, with Command+O and the All notes row.
func (p *App) showAllNotesInFloatingWindow() {
	p.floatPane = floatingPaneState{pane: paneAllNotes}
}

// askToDeleteFloatingNote shows the confirmation for the note the window shows.
func (p *App) askToDeleteFloatingNote() {
	if p.floating != nil {
		p.floatPane.pane = paneConfirmDelete
	}
}

// deleteFloatingNote deletes the note the window shows once it is confirmed; the window shows the next note.
func (p *App) deleteFloatingNote(id string) {
	p.floatPane = floatingPaneState{}
	p.deleteNote(id)
}

// showFloatingNoteFromList shows a note the list chose in the window, and returns the window to the editor.
func (p *App) showFloatingNoteFromList(id string) {
	p.floatPane = floatingPaneState{}
	edit, ok := p.acquireEdit(id)
	if !ok {
		p.Toast("This note no longer exists")
		return
	}
	p.showInFloatingWindow(edit)
}

// floatingListNotes is the list's notes: those whose text contains the search, newest first.
func (p *App) floatingListNotes() []notes.Note {
	return p.noteMatches(p.floatPane.query)
}
