package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// sourceTextPt is the size of the source text in the source pane.
const sourceTextPt = 12

// extensionSourceView shows an extension's source as one scrolling pane, with no rows to choose.
func extensionSourceView(title, source string) *palette.View {
	return &palette.View{
		Title: title + " source", HideInput: true,
		Body: func(c *ui.Context, _ search.Host) {
			t := design.Tokens(c)
			ui.Scroll(c).Grow(1).Padding(12).Children(func() {
				ui.Text(c, source).FontSize(sourceTextPt).TextColor(t.Primary)
			})
		},
	}
}
