package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/commands"
	"github.com/pablopunk/nvm/native/internal/palette/aicommands"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The AI command indicator is a panel at the bottom of the display the cursor is on, like the dictation indicator. It
// never takes the keyboard, so the app the selection is in keeps focus.
const (
	aiPanelWidth     = 300
	aiPanelBottomGap = 96
)

// aiIndicator is the panel the AI commands show their status in. It is created on first use and runs on the UI thread.
type aiIndicator struct {
	app    *App
	window *mygo.Window
	status commands.Status
	height int
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
	w.Update(func() { i.status = status })
	if height := aicommands.Height(status); height != i.height {
		i.height = height
		x, y := aiPanelOrigin(i.app.cursorWorkArea(), height)
		w.SetSize(aiPanelWidth, height)
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
	x, y := aiPanelOrigin(i.app.cursorWorkArea(), i.height)
	i.window = mygo.NewWindow(mygo.WindowOptions{
		Title:             "AI Command",
		Width:             aiPanelWidth,
		Height:            i.height,
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
		Content:           ui.View(func(c *ui.Context) { aicommands.Indicator(c, i.status) }),
	})
	applyIndicatorAppearance(i.window)
	i.window.OnClosed(func() { i.window = nil })
	return i.window
}

// aiPanelOrigin places a panel of height at the bottom centre of area.
func aiPanelOrigin(area platform.Rect, height int) (int, int) {
	return area.X + (area.Width-aiPanelWidth)/2, area.Y + area.Height - height - aiPanelBottomGap
}
