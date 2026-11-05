package design

import "github.com/egoist/mygo/ui/richtext"

// MarkdownSource is the rich text document a Markdown editor shows, and the Markdown that document was last made from
// or read into. A change of the text made elsewhere remakes the document; a typed change is read back as Markdown.
type MarkdownSource struct {
	doc    *richtext.Document
	source string
}

// Sync returns the document that shows text, made when there is none and remade when the text changed since it was
// last made or read.
func (m *MarkdownSource) Sync(text string) *richtext.Document {
	switch {
	case m.doc == nil:
		m.doc = richtext.NewDocument(richtext.ParseMarkdown(text))
	case m.source != text:
		m.doc.SetText(richtext.ParseMarkdown(text))
	}
	m.source = text
	return m.doc
}

// Source is the Markdown the document was last made from or read into.
func (m *MarkdownSource) Source() string {
	return m.source
}

// Read records a typed change: the document now holds markdown, and the next Sync keeps it.
func (m *MarkdownSource) Read(markdown string) {
	m.source = markdown
}
