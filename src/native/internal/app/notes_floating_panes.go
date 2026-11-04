package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
)

// floatingPane is what the floating window shows under its header: the note's editor, the list of notes, or the
// confirmation of a delete. A conflict replaces the editor while it waits, as 1.x's choices do.
type floatingPane uint8

const (
	paneEditor floatingPane = iota
	paneAllNotes
	paneConfirmDelete
)

// floatingPaneState is the pane the window shows, with what the list shows: its search text and the row selected.
// Opening the window, or leaving the list, resets it, as 1.x resets the search on navigation.
type floatingPaneState struct {
	pane     floatingPane
	query    string
	selected int
	// actionsOpen shows the ⌘K menu over the pane; actionSelected is its selected row.
	actionsOpen    bool
	actionSelected int
}

// floatingRow is one row of a window list that acts when clicked. The row is drawn by the palette's row design.
func floatingRow(c *ui.Context, d design.RowData, activate func()) {
	row := ui.Column(c)
	row.Children(func() { design.Row(c, d) })
	if row.Clicked() {
		activate()
	}
}

// floatingEditActions are the rows under the editor, as 1.x's note actions are: New note, All notes, and Delete note
// for the note shown.
func floatingEditActions(c *ui.Context, p *App) {
	ui.Column(c).Children(func() {
		floatingRow(c, design.RowData{Section: "Notes", Title: "New note", Subtitle: "Start a fresh Markdown note", Key: "⌘N", Tile: design.TileData{Icon: "plus"}}, p.createFloatingNote)
		floatingRow(c, design.RowData{Title: "All notes", Subtitle: "Search and switch notes", Key: "⌘O", Tile: design.TileData{Icon: "list"}}, p.showAllNotesInFloatingWindow)
		floatingRow(c, design.RowData{Section: "This note", Title: "Delete note", TintTitle: true, Tone: "red", Tile: design.TileData{Icon: "trash-2", Tone: "red"}}, p.askToDeleteFloatingNote)
	})
}

// floatingAllNotes is the list of notes in the window: a search field, the count, a row that starts a note, then the
// notes newest first. Enter, or a click, shows the note in the window.
func floatingAllNotes(c *ui.Context, p *App) {
	k := design.Tokens(c)
	all := p.notes.List()
	items := p.floatingListNotes()
	p.floatPane.selected = min(p.floatPane.selected, len(items))
	ui.Column(c).Grow(1).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, design.Icon("search")).FontSize(15).TextColor(k.Muted)
			search := ui.TextInput(c, &p.floatPane.query).Grow(1).FontSize(14).TextColor(k.Primary).Placeholder("Find a note").PlaceholderColor(k.Faint).AutoFocus()
			if search.Changed() {
				p.floatPane.selected = 0
			}
		})
		ui.Text(c, floatingNoteCount(len(all))).FontSize(12).TextColor(k.Muted)
		if len(all) == 0 {
			design.EmptyState(c, design.EmptyData{Title: "A clear space for your next idea", Subtitle: "Create a Markdown note and keep it close at hand."})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			floatingRow(c, design.RowData{Title: "New note", Subtitle: "Start a fresh Markdown note", Key: "⌘N", Selected: p.floatPane.selected == 0, Tile: design.TileData{Icon: "plus"}}, p.createFloatingNote)
			for i, n := range items {
				floatingRow(c, floatingNoteRow(n, p.floatPane.selected == i+1), func() { p.showFloatingNoteFromList(n.ID) })
			}
		})
	})
}

// floatingNoteRow is the row of a note in the list: its title, its preview and how long ago it was saved.
func floatingNoteRow(n notes.Note, selected bool) design.RowData {
	return design.RowData{
		Title: n.Title(), Subtitle: notes.Preview(n.Markdown), Accessory: notes.RelativeTime(clock(), n.UpdatedAt),
		Selected: selected, Tile: design.TileData{Icon: "sticky-note", Tone: "yellow"},
	}
}

// floatingNoteCount is the list's subtitle, in the 1.x words.
func floatingNoteCount(n int) string {
	if n == 1 {
		return "1 note · Markdown, local, and yours"
	}
	return fmt.Sprintf("%d notes · Markdown, local, and yours", n)
}

// floatingDeleteConfirm asks before the note is deleted, with the 1.x question and the "Delete note" button.
func floatingDeleteConfirm(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	ui.Column(c).Grow(1).Gap(8).Children(func() {
		ui.Text(c, "Delete note").FontSize(15).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, fmt.Sprintf("Delete “%s”? This cannot be undone.", notes.Title(edit.text))).FontSize(13).TextColor(k.Secondary)
		floatingRow(c, design.RowData{Title: "Delete note", TintTitle: true, Tone: "red", Tile: design.TileData{Icon: "trash-2", Tone: "red"}}, func() { p.deleteFloatingNote(edit.id) })
		floatingRow(c, design.RowData{Title: "Cancel", Tile: design.TileData{Icon: "x"}}, func() { p.floatPane.pane = paneEditor })
	})
}

// floatingConflictChoices lists the choices for unsaved changes that were typed on top of an older version of the
// note: the unsaved text, or the saved one. The choice shows the editor with the text it leaves.
func floatingConflictChoices(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	conflict, _ := edit.sess.Conflict()
	ui.Column(c).Grow(1).Gap(6).Children(func() {
		ui.Text(c, "Unsaved changes found").FontSize(15).FontWeight(600).TextColor(k.Strong)
		ui.Text(c, "This note has unsaved changes from another session.").FontSize(12).TextColor(k.Muted)
		floatingRow(c, design.RowData{Title: "Restore unsaved changes", Subtitle: notes.Preview(conflict.Draft.Content), Tile: design.TileData{Icon: "sticky-note", Tone: "yellow"}},
			func() { p.resolveFloatingConflict(edit, edit.sess.KeepDraft) })
		floatingRow(c, design.RowData{Title: "Keep saved version", Subtitle: notes.Preview(conflict.Saved.Markdown), Tile: design.TileData{Icon: "check"}},
			func() { p.resolveFloatingConflict(edit, edit.sess.KeepSaved) })
	})
}

// resolveFloatingConflict applies a conflict choice; a refused choice says why and stays on the choices.
func (p *App) resolveFloatingConflict(edit *noteEdit, resolve func() error) {
	if err := resolve(); err != nil {
		p.Toast(conflictErrorMessage(err))
	}
}
