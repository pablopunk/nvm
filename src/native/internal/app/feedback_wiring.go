package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The feedback panel sits at the bottom centre of the display the cursor is on, like the other indicators. It never
// takes the keyboard, and the palette's hide does not close it.
const feedbackBottomGap = 96

// feedbackMessage shows a message the palette could not draw because it was hidden.
func (p *App) feedbackMessage(message string) {
	p.showFeedback("Nevermind", message)
}

// showFeedback shows title and message in the HUD. It runs on the UI thread. Without the app's window, which only the
// goldens and tests have, there is no display to show it on, so nothing is shown.
func (p *App) showFeedback(title, message string) {
	if p.panel == nil {
		return
	}
	if p.hud == nil {
		p.hud = feedback.New(p.hudPanel, searchClock, p.hudPosition)
	}
	p.hud.Show(title, message)
}

// hudPanel returns the HUD's window, created on first use and created again once the user has closed it.
func (p *App) hudPanel() feedback.Window {
	if p.hudWindow == nil {
		p.hudWindow = mygo.NewWindow(mygo.WindowOptions{
			Title:             "Nevermind Feedback",
			Width:             feedback.Width,
			Height:            feedback.Height(""),
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
			Content:           ui.View(p.hud.View),
		})
		p.hudWindow.OnClosed(func() { p.hudWindow = nil })
	}
	return p.hudWindow
}

// hudPosition returns the top left corner of a HUD panel of w by h at the bottom centre of the cursor's display.
func (p *App) hudPosition(w, h int) (int, int) {
	return feedbackOrigin(p.cursorWorkArea(), w, h)
}

// feedbackOrigin centres a panel of w by h above the bottom gap of area.
func feedbackOrigin(area platform.Rect, w, h int) (int, int) {
	return area.X + (area.Width-w)/2, area.Y + area.Height - h - feedbackBottomGap
}
