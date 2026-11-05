package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// markdownBody draws the Markdown editor of the page and takes over the text a typing in it changed.
func (ed *editorState) markdownBody(c *ui.Context) {
	doc := ed.rich.Sync(ed.text)
	editor := design.MarkdownEditor(c, doc, ed.placeholder).AutoFocus()
	if editor.Changed() {
		markdown := doc.Text().Markdown()
		ed.rich.Read(markdown)
		ed.text = markdown
	}
}
