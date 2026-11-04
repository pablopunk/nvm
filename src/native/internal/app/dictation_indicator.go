package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
)

// The indicator is a small panel at the bottom of the display the cursor is on. It never takes the keyboard: the app
// the user was in keeps focus, and the paste goes there.
const (
	dictationWindowWidth     = 300
	dictationWindowHeight    = 64
	dictationWindowBottomGap = 96
)

// setDictation shows the indicator with state. It runs on the UI thread.
func (p *App) setDictation(state palindicator.State) {
	w := p.dictationWindowFor()
	w.Update(func() { p.dictationView = state })
	if !w.IsVisible() {
		w.ShowInactive()
	}
}

// hideDictation hides the indicator when it is open. It runs on the UI thread.
func (p *App) hideDictation() {
	if p.dictationWindow != nil {
		p.dictationWindow.Hide()
	}
}

// hideDictationAfter hides the indicator after d, unless another dictation started in the meantime (epoch changed).
func (p *App) hideDictationAfter(epoch int, d time.Duration) {
	time.AfterFunc(d, func() {
		p.redraw(func() {
			if epoch == p.dictationEpoch {
				p.hideDictation()
			}
		})
	})
}

// dictationWindowFor returns the indicator window, creating it on first use. It runs on the UI thread.
func (p *App) dictationWindowFor() *mygo.Window {
	if p.dictationWindow == nil {
		p.dictationWindow = p.newDictationWindow()
	}
	return p.dictationWindow
}

// newDictationWindow builds the hidden indicator panel. Closing it forgets the window; the next dictation makes one.
func (p *App) newDictationWindow() *mygo.Window {
	area := p.cursorWorkArea()
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:             "Dictation",
		Width:             dictationWindowWidth,
		Height:            dictationWindowHeight,
		X:                 area.X + (area.Width-dictationWindowWidth)/2,
		Y:                 area.Y + area.Height - dictationWindowHeight - dictationWindowBottomGap,
		Hidden:            true,
		Frameless:         true,
		Transparent:       true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		Panel:             true,
		DisableResize:     true,
		DisableMove:       true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		Content:           ui.View(func(c *ui.Context) { palindicator.Indicator(c, p.dictationView) }),
	})
	w.OnClosed(func() { p.dictationWindow = nil })
	return w
}
