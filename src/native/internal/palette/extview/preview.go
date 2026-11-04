package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// previewPage is a read-only page of content the extension already has. Its body is Markdown when it has any, else
// plain text, else the view's subtitle. Images and video are not drawn yet.
func (s *Session) previewPage() *palette.View {
	return &palette.View{
		Title: s.view.Title, HideInput: true,
		Body: func(c *ui.Context, _ search.Host) { s.drawPreview(c) },
	}
}

func (s *Session) drawPreview(c *ui.Context) {
	preview := s.view.Preview
	ui.Scroll(c).Grow(1).Children(func() {
		switch {
		case preview != nil && preview.Markdown != "":
			drawMarkdown(c, preview.Markdown, s.markdown)
		case preview != nil && preview.Text != "":
			drawPlainText(c, preview.Text)
		default:
			drawPlainText(c, s.view.Subtitle)
		}
	})
}

// drawNotice is the one-line body of a page the palette cannot fill.
func drawNotice(c *ui.Context, text string) {
	t := design.Tokens(c)
	ui.Column(c).Grow(1).Padding(14).Children(func() {
		ui.Text(c, text).FontSize(11).TextColor(t.Secondary)
	})
}
