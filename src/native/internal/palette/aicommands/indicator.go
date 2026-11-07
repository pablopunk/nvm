package aicommands

import (
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/design"
)

const (
	BaseWidth           = 224
	BaseHeight          = 44
	expandedWidth       = 320
	messageLineHeight   = 16
	messageChromeHeight = 44
)

// Indicator makes the current state primary; the operation remains visible as quieter context.
func Indicator(c *ui.Context, s commands.Status) {
	c.Root().Background(ui.Transparent)
	k := design.Tokens(c)
	tone, icon := k.Accent, "sparkles"
	switch {
	case s.Tone == commands.ToneError:
		tone, icon = k.Danger, "circle-alert"
	case s.Tone == commands.ToneSuccess:
		tone, icon = k.Success, "check"
	case s.Activity == commands.ActivityRecording:
		icon = "mic"
	case s.Activity == commands.ActivityTranscribing:
		icon = "sparkles"
	}
	compact := compactIndicator(s)
	active := s.Tone == commands.ToneLoading && s.Activity != commands.ActivityNone
	phase := float32(0)
	if active {
		phase = design.IndicatorActivity(c, string(s.Activity)+":status", c.Preferences().ReduceMotion)
	}
	pulse := 2 * min(phase, 1-phase)
	design.IndicatorPane(c, func() {
		ui.Row(c).Grow(1).Padding(6, 10).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(24, 24).Radius(12).Background(tone.Alpha(0.12)).Center().Children(func() {
				source := design.Icon(icon)
				if active && s.Activity != commands.ActivityRecording {
					source = design.StatusGlyph("progress")
				}
				glyph := ui.Icon(c, source).FontSize(15).TextColor(tone)
				if active && s.Activity == commands.ActivityRecording && !c.Preferences().ReduceMotion {
					glyph.Opacity(0.78 + 0.22*pulse)
				}
				if active && s.Activity != commands.ActivityRecording && !c.Preferences().ReduceMotion {
					glyph.Rotate(360 * phase)
				}
			})
			primary, context := s.Subtitle, s.Title
			if primary == "" {
				primary, context = context, ""
			}
			if compact {
				ui.Text(c, primary).Grow(1).FontSize(13).FontWeight(500).TextColor(k.Strong).SingleLine().
					ContentTransition(design.IndicatorContentTransition())
				if context != "" {
					ui.Box(c).Width(1).Height(12).Background(k.EdgeStrong)
					ui.Text(c, context).FontSize(11).TextColor(k.Muted).SingleLine()
				}
			} else {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					if context != "" {
						ui.Text(c, context).FontSize(11).TextColor(k.Muted).SingleLine()
					}
					ui.Text(c, primary).FontSize(13).FontWeight(500).TextColor(k.Strong).
						ContentTransition(design.IndicatorContentTransition())
				})
			}
		})
	})
}

func compactIndicator(s commands.Status) bool {
	return s.Tone != commands.ToneError && indicatorColumns(s.Title) <= 18 &&
		indicatorColumns(s.Subtitle) <= 20 && indicatorColumns(s.Title)+indicatorColumns(s.Subtitle) <= 24 &&
		!strings.ContainsAny(s.Title, "\r\n") && !strings.ContainsAny(s.Subtitle, "\r\n")
}

// Width keeps short live states compact and gives explanations room to wrap.
func Width(s commands.Status) int {
	if compactIndicator(s) {
		return BaseWidth
	}
	return expandedWidth
}

// Height reserves full message lines for explanations, including explicit breaks and wide glyphs.
func Height(s commands.Status) int {
	if compactIndicator(s) {
		return BaseHeight
	}
	lines := 0
	for _, line := range strings.Split(s.Subtitle, "\n") {
		lines += max(1, (indicatorColumns(line)+31)/32)
	}
	return max(64, messageChromeHeight+lines*messageLineHeight)
}

func indicatorColumns(text string) int {
	columns := 0
	for _, r := range text {
		columns++
		if r == 'W' || r == 'M' || unicode.Is(unicode.Han, r) || r >= 0x2e80 {
			columns++
		}
	}
	return columns
}
