package design

import "github.com/egoist/mygo/ui"

// PaneRadius is the corner radius of the palette window. The window cut, the pane border and the clip all use
// it, so the corners agree in light, dark, with and without a material.
const PaneRadius = 18

// Pane draws the palette's window pane: a rounded, clipped, bordered column that holds body. With a system
// material (VibrancyGlass) the pane has no fill and the material shows through; without one the pane is the
// theme's paper or ink, slightly see-through where the compositor allows.
func Pane(c *ui.Context, body func()) {
	pane(c, 0.96, body)
}

// PalettePane is the translucent surface for the full palette window.
func PalettePane(c *ui.Context, body func()) {
	pane(c, 0.75, body)
}

func pane(c *ui.Context, opacity float32, body func()) {
	k := Tokens(c)
	pane := ui.Column(c).Fill().Radius(PaneRadius).Clip().Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi)
	if !c.Vibrancy() {
		pane.Background(k.Bg.Alpha(opacity))
	}
	pane.Children(body)
}
