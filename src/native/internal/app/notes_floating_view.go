package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
)

// floatingNoteWindow is the content of the floating window, laid out as the 1.x window is: a strip under the window
// controls that drags the window, then the pane the window shows: the note with its header and actions, a list of
// notes, or a confirmation.
func floatingNoteWindow(c *ui.Context, p *App, edit *noteEdit) {
	p.floatingKeys(c)
	k := design.Tokens(c)
	ui.Themed(c, design.WindowTheme(c.Theme().Dark)).Fill().Background(k.Canvas).Children(func() {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Height(floatingControlsGap).FillWidth().DragWindow()
			ui.Column(c).Grow(1).Padding(0, 12, 12, 12).Gap(8).Children(func() {
				floatingPanes(c, p, edit)
				design.HintBar(c, floatingHints(p, edit))
			})
		})
		if p.floatPane.actionsOpen {
			floatingActionsMenu(c, p, edit)
		}
	})
}

// floatingPanes draws the pane the window shows. While a conflict waits, the note's choices replace its editor.
func floatingPanes(c *ui.Context, p *App, edit *noteEdit) {
	switch p.floatPane.pane {
	case paneAllNotes:
		floatingAllNotes(c, p)
	case paneConfirmDelete:
		floatingDeleteConfirm(c, p, edit)
	default:
		syncBuffer(edit)
		if _, pending := edit.sess.Conflict(); pending {
			floatingConflictChoices(c, p, edit)
			return
		}
		floatingNoteHeader(c, p, edit)
		floatingNoteBody(c, edit)
		floatingEditActions(c, p)
	}
}

// floatingNoteHeader is the note's title over its state: "Markdown · Edited 3m ago", as 1.x's editor header shows.
// The row drags the window, as the 1.x strip does.
func floatingNoteHeader(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	title, subtitle := notes.Title(edit.text), "Markdown"
	if n, ok := p.notes.Get(edit.id); ok {
		subtitle += " · Edited " + notes.RelativeTime(clock(), n.UpdatedAt)
	}
	ui.Row(c).Padding(2, 4, 0, 4).Gap(10).AlignItems(ui.Baseline).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.Primary).SingleLine().Grow(1)
		ui.Text(c, subtitle).FontSize(12).TextColor(k.Muted).SingleLine()
	})
}

// floatingNoteBody is the note's Markdown in the rich text editor, with the same autosave as the editor.
func floatingNoteBody(c *ui.Context, edit *noteEdit) {
	ui.Column(c).Grow(1).Padding(8, 4).Children(func() {
		editNoteRich(c, edit, &edit.floatRich, editorPlaceholder)
	})
}
