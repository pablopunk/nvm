package design

import "github.com/egoist/mygo/ui"

// Placement is where a popover floats inside its pane: the distance from the right and the bottom edge, and
// its width, in DIPs.
type Placement struct {
	Right, Bottom, Width float32
}

// ActionPlacement is where the action menu floats: over the footer at the right edge of the pane.
var ActionPlacement = Placement{Right: 12, Bottom: 44, Width: 290}

// Popover floats body over its pane at place: a raised, rounded surface with a lit top edge and a soft
// shadow, so it reads as lifted above the rows it covers.
func Popover(c *ui.Context, place Placement, body func()) {
	k := Tokens(c)
	ui.Column(c).Absolute().Right(place.Right).Bottom(place.Bottom).Width(place.Width).Radius(12).
		Background(k.Raised).Border(1, k.EdgeStrong).InsetShadow(0, 1, 0, 0, k.Hi).
		Shadow(0, 14, 34, 0, k.Shadow).Shadow(0, 2, 6, 0, k.Lo).Padding(5).Children(body)
}
