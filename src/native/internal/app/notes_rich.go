package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/pablopunk/nvm/native/internal/design"
)

// noteRichText is the rich text one view edits a note with: the document its editor shows, and the Markdown that
// document was last made from or read into. Each view has its own, so the palette's editor and the floating window
// can both show a note; the note's text is the one they share.
type noteRichText struct {
	doc    *richtext.Document
	source string
}

// sync returns the document showing text, made when there is none and remade when the text changed since it was
// last made or read, as a typing in another view does.
func (r *noteRichText) sync(text string) *richtext.Document {
	switch {
	case r.doc == nil:
		r.doc = richtext.NewDocument(richtext.ParseMarkdown(text))
	case r.source != text:
		r.doc.SetText(richtext.ParseMarkdown(text))
	}
	r.source = text
	return r.doc
}

// editNoteRich draws the Markdown editor of a note and, after an edit, saves the Markdown as the note's text.
func editNoteRich(c *ui.Context, edit *noteEdit, rich *noteRichText, placeholder string) {
	doc := rich.sync(edit.text)
	editor := design.MarkdownEditor(c, doc, placeholder).AutoFocus()
	if editor.Changed() {
		markdown := doc.Text().Markdown()
		rich.source = markdown
		edit.text = markdown
		_ = edit.sess.Edit(markdown)
	}
}
