package feedback

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// Panel heights: a title and one line of text, then one more line for every charsPerLine characters of the message,
// up to maxLines, so a long message wraps instead of being cut. A status mark takes width, so its text wraps sooner.
const (
	baseHeight         = 56
	lineHeight         = 18
	charsPerLine       = 44
	charsPerLineStatus = 40
	maxLines           = 3
)

// Height returns the panel height that fits message read as tone.
func Height(message string, tone Tone) int {
	perLine := charsPerLine
	if statusKind(tone) != "" {
		perLine = charsPerLineStatus
	}
	lines := (utf8.RuneCountInString(message) + perLine - 1) / perLine
	lines = min(max(lines, 1), maxLines)
	return baseHeight + (lines-1)*lineHeight
}

// statusKind is the status mark a tone shows: success and error have one, the default tone has none.
func statusKind(tone Tone) string {
	switch tone {
	case ToneSuccess:
		return "success"
	case ToneError:
		return "error"
	}
	return ""
}

// statusColor is the color of a tone's status mark.
func statusColor(k *design.TokenSet, tone Tone) ui.Color {
	if tone == ToneError {
		return k.Danger
	}
	return k.Success
}

// Indicator draws the HUD's panel: the title small and muted, the message below it. A success or an error puts its
// status mark at the left of the text; the default tone draws the text alone.
func Indicator(c *ui.Context, title, message string, tone Tone) {
	k := design.Tokens(c)
	c.Root().Background(ui.Transparent)
	design.Pane(c, func() {
		ui.Column(c).Grow(1).MinHeight(0).Padding(10, 14).Gap(2).Children(func() {
			glyph := design.StatusGlyph(statusKind(tone))
			if glyph == nil {
				indicatorText(c, k, title, message)
				return
			}
			ui.Row(c).Grow(1).MinHeight(0).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, glyph).FontSize(20).TextColor(statusColor(k, tone))
				ui.Column(c).Grow(1).MinHeight(0).Gap(2).Children(func() { indicatorText(c, k, title, message) })
			})
		})
	})
}

// indicatorText draws the title and the message of the panel.
func indicatorText(c *ui.Context, k *design.TokenSet, title, message string) {
	ui.Text(c, title).FontSize(11).FontWeight(600).TextColor(k.Muted).SingleLine()
	ui.Text(c, message).FontSize(13).TextColor(k.Strong)
}
