package aicommands

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/design"
)

// The indicator panel's base height, and the lines a failure's copy wraps to at the panel's width. A failure grows the
// panel so the whole message shows, as the dictation indicator does.
const (
	BaseHeight          = 64
	failureChromeHeight = 40
	failureLineHeight   = 15
	failureCharsPerLine = 42
)

// Indicator draws the status pane of an AI command: the command's title and one line of copy in the tone's colour.
func Indicator(c *ui.Context, s commands.Status) {
	c.Root().Background(ui.Transparent)
	k := design.Tokens(c)
	tone, subtitleColor := k.Accent, k.Muted
	icon := "sparkles"
	listening := strings.Contains(strings.ToLower(s.Title+" "+s.Subtitle), "dictat") && s.Tone == commands.ToneLoading
	if listening {
		icon = "mic"
	}
	switch s.Tone {
	case commands.ToneSuccess:
		tone = k.Tones["green"]
	case commands.ToneError:
		tone, subtitleColor = k.Danger, k.Danger
		icon = "circle-alert"
	}
	design.Pane(c, func() {
		ui.Row(c).Grow(1).Padding(10, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			if listening {
				// The native indicator reports truthful activity without implying microphone amplitude.
				progress := ui.Row(c).Key("dictation-activity").Keyframes("activity", []ui.Keyframe{
					{Value: 0.72}, {At: 420 * time.Millisecond, Value: 1, Ease: ui.EaseInOut},
				})
				ui.Icon(c, design.Icon(icon)).FontSize(16).TextColor(tone).Opacity(progress)
			} else {
				ui.Icon(c, design.Icon(icon)).FontSize(16).TextColor(tone)
			}
			ui.Column(c).Grow(1).MinHeight(0).Children(func() {
				ui.Text(c, s.Title).FontSize(13).FontWeight(600).TextColor(k.Strong).SingleLine().ContentTransition(design.IndicatorContentTransition())
				subtitle := ui.Text(c, s.Subtitle).FontSize(11).TextColor(subtitleColor).ContentTransition(design.IndicatorContentTransition())
				if s.Tone != commands.ToneError {
					subtitle.SingleLine()
				}
			})
		})
	})
}

// Height is the panel height that fits s: the base height, or more when a failure's copy wraps onto more lines.
func Height(s commands.Status) int {
	if s.Tone != commands.ToneError {
		return BaseHeight
	}
	lines := max(1, (utf8.RuneCountInString(s.Subtitle)+failureCharsPerLine-1)/failureCharsPerLine)
	return max(BaseHeight, failureChromeHeight+lines*failureLineHeight)
}
