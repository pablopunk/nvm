package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// livePreviewBody draws the raw Markdown source beside its rendered preview.
func (ed *editorState) livePreviewBody(c *ui.Context) {
	ui.Row(c).Grow(1).Margin(12).Gap(12).Children(func() {
		ui.TextArea(c, &ed.text).Grow(1).Padding(10).FontSize(14).Placeholder(ed.placeholder).ReadOnly(ed.readOnly).AutoFocus().Value(ed.text)
		ui.Column(c).Grow(1).Padding(10).Children(func() { drawMarkdown(c, ed.text, ed.sess.markdown) })
	})
}

// markdownBody draws the Markdown editor of the page and takes over the text a typing in it changed.
func (ed *editorState) markdownBody(c *ui.Context) {
	doc := ed.rich.Sync(ed.text)
	editor := design.MarkdownEditor(c, doc, ed.placeholder).Bind(&ed.focus).AutoFocus()
	if editor.Changed() {
		markdown := doc.Text().Markdown()
		ed.rich.Read(markdown)
		ed.text = markdown
	}
}
