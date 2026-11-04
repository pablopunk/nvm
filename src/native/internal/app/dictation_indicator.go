package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	palindicator "github.com/pablopunk/nvm/native/internal/palette/dictation"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The indicator is a small panel at the bottom of the display the cursor is on. It never takes the keyboard: the app
// the user was in keeps focus, and the paste goes there. Its height follows the state, so a failure's whole message
// shows.
const (
	dictationWindowWidth     = 300
	dictationWindowBottomGap = 96
)

// dictationWindow is the flow's panel: a mygo window created on first use, closed by the user or hidden between
// dictations. It runs on the UI thread.
type dictationWindow struct {
	app    *App
	height int
}

// Show sets the indicator to state, sizes the panel to fit it and shows it without taking focus.
func (d *dictationWindow) Show(state palindicator.State) {
	p := d.app
	w := d.windowFor()
	w.Update(func() { p.dictationView = state })
	if height := palindicator.Height(state); height != d.height {
		d.height = height
		x, y := dictationWindowOrigin(p.cursorWorkArea(), height)
		w.SetSize(dictationWindowWidth, height)
		w.SetPosition(x, y)
	}
	if !w.IsVisible() {
		w.ShowInactive()
	}
}

// Hide closes the panel when it is open.
func (d *dictationWindow) Hide() {
	if d.app.dictationWindow != nil {
		d.app.dictationWindow.Hide()
	}
}

// windowFor returns the panel's window, creating it on first use.
func (d *dictationWindow) windowFor() *mygo.Window {
	p := d.app
	if p.dictationWindow == nil {
		d.height = palindicator.BaseHeight
		x, y := dictationWindowOrigin(p.cursorWorkArea(), d.height)
		p.dictationWindow = mygo.NewWindow(mygo.WindowOptions{
			Title:             "Dictation",
			Width:             dictationWindowWidth,
			Height:            d.height,
			X:                 x,
			Y:                 y,
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
			Content:           ui.View(func(c *ui.Context) { p.pressDictationControl(palindicator.Indicator(c, p.dictationView)) }),
		})
		p.dictationWindow.OnClosed(func() { p.dictationWindow = nil })
	}
	return p.dictationWindow
}

// dictationWindowOrigin places a panel of height at the bottom centre of area.
func dictationWindowOrigin(area platform.Rect, height int) (int, int) {
	x := area.X + (area.Width-dictationWindowWidth)/2
	y := area.Y + area.Height - height - dictationWindowBottomGap
	return x, y
}

// pressDictationControl runs the button pressed in this frame on the UI thread after the frame, since the flow hides
// the panel the press came from.
func (p *App) pressDictationControl(control palindicator.Control) {
	if control == palindicator.ControlNone {
		return
	}
	p.dictationAfter(0, func() { p.dictationFlowFor().Press(control) })
}
