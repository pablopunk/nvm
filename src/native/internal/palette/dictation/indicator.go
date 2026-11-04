// Package dictation draws the dictation indicator, the small panel that shows a dictation while the palette is
// hidden: listening with a level meter, waiting for the microphone, transcribing, and the two ways it ends
// without text. It draws a State and holds nothing, so the composition root decides what the user sees.
package dictation

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// Mode is the part of a dictation the indicator shows.
type Mode int

// The indicator modes, in the order a dictation passes through them. NoSpeech and Failed end a dictation.
const (
	ModeListening Mode = iota
	ModeWaitingForMicrophone
	ModeTranscribing
	ModeNoSpeech
	ModeFailed
)

// State is what one indicator frame draws. Level is the meter's value from 0 to 1, read only in ModeListening.
// Detail is the failure in words the user can act on, read only in ModeFailed.
type State struct {
	Mode   Mode
	Level  float64
	Detail string
}

const (
	meterBars      = 12
	meterMaxHeight = 22
	meterBarWidth  = 3
)

// meterWeights shapes the bars so the meter reads as a voice spectrum rather than a flat block.
var meterWeights = [meterBars]float64{0.45, 0.65, 0.85, 1, 0.8, 0.6, 0.9, 0.7, 0.5, 0.75, 0.55, 0.4}

// Indicator draws the indicator's pane for s.
func Indicator(c *ui.Context, s State) {
	k := design.Tokens(c)
	tone := k.Accent
	subtitleColor := k.Muted
	if s.Mode == ModeFailed {
		tone = k.Danger
		subtitleColor = k.Danger
	}
	design.Pane(c, func() {
		ui.Row(c).Grow(1).Padding(10, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, design.Icon("mic")).FontSize(16).TextColor(tone)
			ui.Column(c).Grow(1).MinHeight(0).Children(func() {
				ui.Text(c, "Dictation").FontSize(13).FontWeight(600).TextColor(k.Strong).SingleLine()
				ui.Text(c, Subtitle(s)).FontSize(11).TextColor(subtitleColor).SingleLine()
			})
			if s.Mode == ModeListening {
				meter(c, s.Level, tone)
			}
		})
	})
}

// Subtitle is the line under the title. Each mode has one line of copy, and failures carry their own detail.
func Subtitle(s State) string {
	switch s.Mode {
	case ModeListening:
		return "Listening"
	case ModeWaitingForMicrophone:
		return "Waiting for microphone"
	case ModeTranscribing:
		return "Transcribing"
	case ModeNoSpeech:
		return "No speech detected"
	case ModeFailed:
		return s.Detail
	}
	return ""
}

// meter draws the level as a row of bars, each scaled by its weight and never shorter than a dot.
func meter(c *ui.Context, level float64, tone ui.Color) {
	ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
		for _, weight := range meterWeights {
			height := max(3, float32(meterMaxHeight*clampLevel(level)*weight))
			ui.Column(c).Width(meterBarWidth).Height(height).Radius(2).Background(tone)
		}
	})
}

func clampLevel(level float64) float64 {
	return min(1, max(0, level))
}
