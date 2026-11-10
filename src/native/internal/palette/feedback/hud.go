// Package feedback is the HUD that tells the user what an action did while the palette is hidden: a title, a line of
// text, and then it goes away on its own. The composition root owns the window and where it goes; the HUD decides what
// it shows and when it hides.
package feedback

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/design"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Width is the panel's width in DIPs; its height follows the message.
const Width = 320

// Tone is how a message reads. An error stays up longer than the rest, as 1.x's feedback does.
type Tone int

// The tones a message can have.
const (
	ToneDefault Tone = iota
	ToneSuccess
	ToneError
)

// Duration is how long a message without an error stays on the HUD, as 1.x's feedback does.
const Duration = 2200 * time.Millisecond

// ErrorDuration is how long an error stays on the HUD, as 1.x's feedback does.
const ErrorDuration = 4000 * time.Millisecond

// DurationOf returns how long a message of tone stays on the HUD.
func DurationOf(tone Tone) time.Duration {
	if tone == ToneError {
		return ErrorDuration
	}
	return Duration
}

// Window is the HUD's panel. The composition root creates it and it runs on the UI thread, as the palette's does.
type Window interface {
	Update(fn func())
	SetSize(w, h int)
	SetPosition(x, y int)
	ShowInactive()
	Hide()
	IsVisible() bool
}

// HUD shows messages in its panel, one at a time. It is used on the UI thread only, except for the hide, which the
// clock runs on its own goroutine and hands back to the UI thread.
type HUD struct {
	panel    func() Window
	clock    platform.Clock
	position func(w, h int) (x, y int)
	// title, message and tone are what the panel draws; generation counts the messages shown, so a hide scheduled for an
	// earlier message does not close a newer one.
	title, message string
	tone           Tone
	generation     int
	height         int
	entry          uint64
}

// New returns a HUD whose panel comes from panel each time it shows, so the composition root can create it on
// first use and drop it when the user closes it. position returns the top left corner for a panel of w by h.
func New(panel func() Window, clock platform.Clock, position func(w, h int) (x, y int)) *HUD {
	return &HUD{panel: panel, clock: clock, position: position}
}

// Show puts title and message on the panel, sizes the panel to fit the message, shows it without taking the keyboard,
// and hides it after the duration of tone. A message that arrives while another is up replaces it and starts the wait
// again.
func (h *HUD) Show(title, message string, tone Tone) {
	w := h.panel()
	h.generation++
	generation := h.generation
	entering := !w.IsVisible()
	w.Update(func() {
		if entering {
			h.entry++
		}
		h.draw(title, message, tone)
	})
	if height := Height(message, tone); height != h.height {
		h.height = height
		w.SetSize(Width, height)
		x, y := h.position(Width, height)
		w.SetPosition(x, y)
	}
	if !w.IsVisible() {
		w.ShowInactive()
	}
	go h.hideAfterDuration(w, generation, DurationOf(tone))
}

// hideAfterDuration waits for d and hides the panel, unless a newer message has taken it over.
func (h *HUD) hideAfterDuration(w Window, generation int, d time.Duration) {
	<-h.clock.After(d)
	w.Update(func() {
		if generation == h.generation {
			w.Hide()
		}
	})
}

// draw keeps the text the panel shows, for the next frame.
func (h *HUD) draw(title, message string, tone Tone) {
	h.title, h.message, h.tone = title, message, tone
}

// View draws the message the HUD holds. The panel's content is this view.
func (h *HUD) View(c *ui.Context) {
	design.IndicatorEntrance(c, h.entry, func() { Indicator(c, h.title, h.message, h.tone) })
}
