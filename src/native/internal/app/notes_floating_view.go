package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
)

// floatingNoteWindow is the content of the floating window, laid out as the 1.x window is: a strip under the window
// controls that drags the window, then the note's header (its title and when it was edited), then the editor.
func floatingNoteWindow(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	ui.Themed(c, design.WindowTheme(c.Theme().Dark)).Fill().Background(k.Canvas).Children(func() {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Height(floatingControlsGap).Fill().DragWindow()
			ui.Column(c).Grow(1).Padding(0, 12, 12, 12).Gap(8).Children(func() {
				floatingNoteHeader(c, p, edit)
				floatingNoteBody(c, edit)
			})
		})
	})
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

// floatingNoteBody is the note's Markdown in the rich text editor, with the same autosave as the editor. While a
// conflict waits, it says so instead.
func floatingNoteBody(c *ui.Context, edit *noteEdit) {
	syncBuffer(edit)
	if _, pending := edit.sess.Conflict(); pending {
		conflictNotice(c, "Unsaved changes from another session are waiting. Open this note in Quick Notes to choose which version to keep.")
		return
	}
	doc := noteDocument(edit)
	ui.Column(c).Grow(1).Padding(8, 4).Children(func() {
		editor := design.MarkdownEditor(c, doc, editorPlaceholder).AutoFocus()
		if editor.Changed() {
			markdown := doc.Text().Markdown()
			edit.text, edit.docSource = markdown, markdown
			_ = edit.sess.Edit(markdown)
		}
	})
}

// noteDocument returns the rich text of a note, made from its text when it has none yet and remade when the text
// changed under it, as a palette editor's typing or a conflict choice does.
func noteDocument(edit *noteEdit) *richtext.Document {
	switch {
	case edit.doc == nil:
		edit.doc = richtext.NewDocument(richtext.ParseMarkdown(edit.text))
	case edit.docSource != edit.text:
		edit.doc.SetText(richtext.ParseMarkdown(edit.text))
	}
	edit.docSource = edit.text
	return edit.doc
}
