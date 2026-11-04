package aicommands

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// copyHint is the footer of a result page: Enter and the copy key both copy the text.
const copyHint = "↵ copy · ⌘⌥C copy"

// ResultView is the page of a finished command: its text, read on every frame, and the keys that copy it. The palette
// input is not used for anything; Escape closes the page.
func ResultView(r commands.Result) *palette.View {
	var host search.Host
	return &palette.View{
		Title: r.Title,
		Body: func(c *ui.Context, h search.Host) {
			host = h
			drawResult(c, r.Text())
		},
		OnKey: func(c *ui.Context, h search.Host) bool {
			host = h
			if !c.Shortcut(palette.Mod|ui.Alt, ui.KeyC) {
				return false
			}
			copyResult(h, r)
			return true
		},
		OnSubmit: func(string) { copyResult(host, r) },
		Hint:     copyHint,
	}
}

// drawResult draws the text of a result, wrapped to the pane's width.
func drawResult(c *ui.Context, text string) {
	k := design.Tokens(c)
	ui.Column(c).Grow(1).Padding(12, 16).Children(func() {
		ui.Text(c, text).FontSize(14).TextColor(k.Strong)
	})
}

// copyResult copies the result text and closes the page, the way a copy action does.
func copyResult(h search.Host, r commands.Result) {
	if h != nil {
		h.Hide()
	}
	if r.Copy != nil {
		r.Copy(r.Text())
	}
}
