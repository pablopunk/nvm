package design

import "github.com/egoist/mygo/ui"

// IndicatorRadius keeps the native material and its content on the same outline.
const IndicatorRadius = 20

// IndicatorPane is a quiet floating surface for transient status, with space for its contact shadow.
func IndicatorPane(c *ui.Context, body func()) {
	k := Tokens(c)
	pane := ui.Column(c).Fill().Radius(IndicatorRadius).Clip().Role(ui.RoleStatus)
	if c.Vibrancy() {
		pane.Background(k.Surface.Alpha(0.08))
	} else {
		pane.Margin(3).Background(k.Bg.Alpha(0.94)).Border(1, k.EdgeSubtle).
			InsetShadow(0, 1, 0, 0, k.Hi).Shadow(0, 1, 3, 0, k.Lo)
	}
	pane.Children(body)
}
