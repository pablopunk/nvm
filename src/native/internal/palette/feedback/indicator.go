package feedback

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// Panel heights: a title and one line of text, then one more line for every charsPerLine characters of the message,
// up to maxLines, so a long message wraps instead of being cut.
const (
	baseHeight   = 56
	lineHeight   = 18
	charsPerLine = 44
	maxLines     = 3
)

// Height returns the panel height that fits message.
func Height(message string) int {
	lines := (utf8.RuneCountInString(message) + charsPerLine - 1) / charsPerLine
	lines = min(max(lines, 1), maxLines)
	return baseHeight + (lines-1)*lineHeight
}

// Indicator draws the HUD's panel: the title small and muted, the message below it.
func Indicator(c *ui.Context, title, message string) {
	k := design.Tokens(c)
	c.Root().Background(ui.Transparent)
	design.Pane(c, func() {
		ui.Column(c).Grow(1).MinHeight(0).Padding(10, 14).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(11).FontWeight(600).TextColor(k.Muted).SingleLine()
			ui.Text(c, message).FontSize(13).TextColor(k.Strong)
		})
	})
}
