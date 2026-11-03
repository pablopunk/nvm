package design

import "github.com/egoist/mygo/ui"

// Depth rules: flat fills, a lit top edge, a soft shadow below raised things, a groove for dividers. The
// only gradient is the gold glossy key of a prominent control. Surfaces are told apart by light, not by
// heavy outlines.

func mix(a, b ui.Color, t float32) ui.Color {
	m := func(x, y uint8) uint8 { return uint8(float32(x)*(1-t) + float32(y)*t) }
	return ui.Color{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: a.A}
}

func lighten(c ui.Color, t float32) ui.Color { return mix(c, ui.RGB(255, 255, 255), t) }
func darken(c ui.Color, t float32) ui.Color  { return mix(c, ui.RGB(0, 0, 0), t) }

// Etched draws a groove: a dark hairline with a light one under it, as a divider between two regions.
func Etched(c *ui.Context) {
	k := Tokens(c)
	ui.Box(c).Height(1).Background(k.Lo)
	ui.Box(c).Height(1).Background(k.Hi)
}

// Glossy fills a colored control with a vertical sheen, a lit top edge and a soft contact shadow.
func Glossy(e ui.Element, col ui.Color, radius float32, shadow float32) ui.Element {
	return e.Radius(radius).Gradient(lighten(col, 0.04), col, 180).
		Border(1, darken(col, 0.22).Alpha(0.35)).
		InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.22)).
		Shadow(0, 1, 2+shadow, 0, ui.RGBA(0, 0, 0, 0.14))
}

// SelectedFill is the highlighted row: a lit warm plate with a faint gold edge, stronger when prominent.
func SelectedFill(e ui.Element, k *TokenSet, prominent bool) ui.Element {
	if prominent {
		return e.Background(k.AccentMuted).
			Border(1, k.AccentEmphasis).InsetShadow(0, 1, 0, 0, k.Hi).Shadow(0, 1, 3, 0, k.Lo)
	}
	return e.Background(k.Selected).Border(1, k.EdgeSubtle).
		InsetShadow(0, 1, 0, 0, k.Hi).Shadow(0, 1, 3, 0, k.Lo)
}
