package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/palette/aicommands"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The AI command indicator is a panel at the bottom of the display the cursor is on, like the dictation indicator. It
// never takes the keyboard, so the app the selection is in keeps focus.
const (
	aiPanelBottomGap = 96
)

// aiIndicator is the panel the AI commands show their status in. It is created on first use and runs on the UI thread.
type aiIndicator struct {
	app    *App
	window *mygo.Window
	status commands.Status
	width  int
	height int
	entry  uint64
}

// aiPanelFor returns the app's AI indicator, created on first use.
func (p *App) aiPanelFor() *aiIndicator {
	if p.aiPanel == nil {
		p.aiPanel = &aiIndicator{app: p}
	}
	return p.aiPanel
}

// show sets the panel to status, sizes it to fit and shows it without taking focus.
func (i *aiIndicator) show(status commands.Status) {
	w := i.windowFor()
	entering := !w.IsVisible()
	w.Update(func() {
		if entering {
			i.entry++
		}
		i.status = status
	})
	if width, height := aicommands.Width(status), aicommands.Height(status); width != i.width || height != i.height {
		i.width, i.height = width, height
		x, y := aiPanelOrigin(i.app.cursorWorkArea(), width, height)
		w.SetSize(width, height)
		w.SetPosition(x, y)
	}
	if !w.IsVisible() {
		w.ShowInactive()
	}
}

// hide closes the panel when it is open.
func (i *aiIndicator) hide() {
	if i.window != nil {
		i.window.Hide()
	}
}

// windowFor returns the panel's window, creating it on first use.
func (i *aiIndicator) windowFor() *mygo.Window {
	if i.window != nil {
		return i.window
	}
	i.height = aicommands.BaseHeight
	i.width = aicommands.BaseWidth
	x, y := aiPanelOrigin(i.app.cursorWorkArea(), i.width, i.height)
	i.window = mygo.NewWindow(mygo.WindowOptions{
		Title:             "AI Command",
		Width:             i.width,
		Height:            i.height,
		X:                 x,
		Y:                 y,
		Hidden:            true,
		Frameless:         true,
		Transparent:       true,
		CornerRadius:      design.IndicatorRadius,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		Panel:             true,
		DisableResize:     true,
		DisableMove:       true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		Content: ui.View(func(c *ui.Context) {
			design.IndicatorEntrance(c, i.entry, func() { aicommands.Indicator(c, i.status) })
		}),
	})
	applyIndicatorAppearance(i.window)
	i.window.OnClosed(func() { i.window = nil })
	return i.window
}

// aiPanelOrigin places a panel of height at the bottom centre of area.
func aiPanelOrigin(area platform.Rect, width, height int) (int, int) {
	return area.X + (area.Width-width)/2, area.Y + area.Height - height - aiPanelBottomGap
}
