package platform

// Point is a position in screen coordinates, in DIPs, with the origin at the primary display's top-left.
type Point struct{ X, Y int }

// Rect is a position and size in screen coordinates.
type Rect struct{ X, Y, Width, Height int }

// Contains reports whether p lies inside r, with the right and bottom edges excluded.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.Width && p.Y < r.Y+r.Height
}

// Display describes one monitor.
type Display struct {
	ID    int64
	Label string
	// Bounds is the whole monitor; WorkArea excludes the menu bar, Dock and taskbars.
	Bounds   Rect
	WorkArea Rect
	// ScaleFactor is the ratio of pixels to DIPs.
	ScaleFactor float64
	Primary     bool
}

// Screens is the intent "know where the user is looking": the monitors, where the pointer is, and which
// monitor a point belongs to, so a window can open on the display the user is using. Every OS supports
// it; mixed-DPI and full-screen behavior differ and are verified per OS in the rig.
type Screens interface {
	// Displays lists the connected monitors.
	Displays() []Display
	// CursorPoint returns where the pointer is.
	CursorPoint() Point
	// NearestDisplay returns the monitor that contains p, or the nearest one when p is off every monitor.
	NearestDisplay(p Point) Display
}
