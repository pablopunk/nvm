package real

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// screens reads monitors and the pointer through MyGo.
type screens struct{}

// Displays lists the connected monitors.
func (screens) Displays() []platform.Display {
	found := mygo.Screen.Displays()
	displays := make([]platform.Display, 0, len(found))
	for _, display := range found {
		displays = append(displays, convertDisplay(display))
	}
	return displays
}

// CursorPoint returns the pointer position.
func (screens) CursorPoint() platform.Point {
	return convertPoint(mygo.Screen.CursorScreenPoint())
}

// NearestDisplay returns the monitor containing p, or the nearest one.
func (screens) NearestDisplay(p platform.Point) platform.Display {
	return convertDisplay(mygo.Screen.DisplayNearestPoint(mygo.Point{X: p.X, Y: p.Y}))
}

// convertDisplay copies a MyGo display into the port's shape.
func convertDisplay(d mygo.Display) platform.Display {
	return platform.Display{
		ID:          d.ID,
		Label:       d.Label,
		Bounds:      convertRect(d.Bounds),
		WorkArea:    convertRect(d.WorkArea),
		ScaleFactor: d.ScaleFactor,
		Primary:     d.Primary,
	}
}

// convertRect copies a MyGo rectangle.
func convertRect(r mygo.Rectangle) platform.Rect {
	return platform.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// convertPoint copies a MyGo point.
func convertPoint(p mygo.Point) platform.Point {
	return platform.Point{X: p.X, Y: p.Y}
}
