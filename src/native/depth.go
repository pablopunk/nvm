package main

import "github.com/egoist/mygo/ui"

// Depth helpers: the palette is a stack of surfaces told apart by light, not by flat fills:
// a faint highlight on every top edge, a darker line under it, soft shadows below raised things.

func mix(a, b ui.Color, t float32) ui.Color {
	m := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t) }
	return ui.Color{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: a.A}
}

func lighten(c ui.Color, t float32) ui.Color { return mix(c, ui.RGB(255, 255, 255), t) }
func darken(c ui.Color, t float32) ui.Color  { return mix(c, ui.RGB(0, 0, 0), t) }

// etched is a groove: a dark hairline with a light one under it.
func etched(c *ui.Context, k *tokens) {
	ui.Box(c).Height(1).Background(k.lo)
	ui.Box(c).Height(1).Background(k.hi)
}

// vetched is the vertical groove.
func vetched(c *ui.Context, k *tokens) {
	ui.Row(c).FillHeight().Children(func() {
		ui.Box(c).Width(1).FillHeight().Background(k.lo)
		ui.Box(c).Width(1).FillHeight().Background(k.hi)
	})
}

// glossy fills a colored control with a vertical sheen, a lit top edge and a soft contact shadow.
func glossy(e ui.Element, col ui.Color, radius float32, shadow float32) ui.Element {
	return e.Radius(radius).Gradient(lighten(col, 0.04), col, 180).
		Border(1, darken(col, 0.22).Alpha(0.35)).
		InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.22)).
		Shadow(0, 1, 2+shadow, 0, ui.RGBA(0, 0, 0, 0.14))
}

// selectedFill is the highlighted row: a lit warm plate with a faint gold edge (stronger when prominent).
func selectedFill(e ui.Element, k *tokens, prominent bool) ui.Element {
	if prominent {
		return e.Background(k.accentMuted).
			Border(1, k.accentEmphasis).InsetShadow(0, 1, 0, 0, k.hi).Shadow(0, 1, 3, 0, k.lo)
	}
	return e.Background(k.selected).Border(1, k.edgeSubtle).
		InsetShadow(0, 1, 0, 0, k.hi).Shadow(0, 1, 3, 0, k.lo)
}
