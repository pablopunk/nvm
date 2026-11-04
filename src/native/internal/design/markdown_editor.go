package design

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// markdownEditorFontSize is the note text's size in DIPs: the 1.x editor set 14px, with a 1.65 line height.
const (
	markdownEditorFontSize   = 14
	markdownEditorLineHeight = 1.65
)

// MarkdownEditor draws a Markdown document as the 1.x editor does: headings, bold, italic, code, lists, quotations
// and links are styled as they are typed, and the Markdown shortcuts ("# ", "**text**", "- ") turn the source into
// its style. doc is the app's, kept between frames; its text is read back as Markdown with Text().Markdown(). The
// editor has no room or background of its own: the caller pads it.
func MarkdownEditor(c *ui.Context, doc *richtext.Document, placeholder string) ui.Element {
	return richtext.EditorBase(c, doc, richtext.EditorOptions{
		Label:       "Note",
		Placeholder: placeholder,
		Font:        ui.Font{Size: markdownEditorFontSize, LineHeight: markdownEditorLineHeight},
		Theme:       markdownEditorTheme(c),
	}).Fill()
}

// markdownEditorTheme sets the editor's colors and spacing to the 1.x editor's CSS. Its lengths are in ems of the
// 14px text, so 8px is 8/14 of an em.
func markdownEditorTheme(c *ui.Context) *richtext.Theme {
	k := Tokens(c)
	th := richtext.DefaultTheme(c.Theme())
	th.Text, th.Caret, th.Code, th.Marker = k.Primary, k.Primary, k.Primary, k.Primary
	th.Muted, th.Link = k.Muted, k.Link
	th.CodeSize = 0.9
	th.CodeBackground, th.CodeBlockBackground = k.Raised, k.Sunken
	th.CodeBlockRadius = em(8)
	th.CodeBlockPadding = [2]float32{em(12), em(8)}
	th.QuoteBar, th.QuoteText, th.QuoteBarWidth = k.EdgeStrong, k.Secondary, 2
	th.Indent = em(24)
	th.ParagraphSpacing = em(8)
	for i, size := range [6]float32{18, 16, 14, 14, 14, 14} {
		th.Headings[i] = richtext.HeadingStyle{Size: size / markdownEditorFontSize, Weight: 700, Color: k.Strong, Bottom: em(8)}
	}
	return th
}

// em converts a length in DIPs to ems of the editor's text.
func em(px float32) float32 {
	return px / markdownEditorFontSize
}
