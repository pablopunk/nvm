package geometry

import "math"

// Point is a position in desktop points.
type Point struct {
	X, Y float64
}

// Size is a width and height in desktop points.
type Size struct {
	Width, Height float64
}

// Rect is a window frame or work area in desktop points. Origins may be negative for displays
// placed left of or above the primary display.
type Rect struct {
	X, Y, Width, Height float64
}

// Center returns the midpoint of the rectangle.
func (r Rect) Center() Point {
	return Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

// HasArea reports whether both dimensions are positive.
func (r Rect) HasArea() bool {
	return r.Width > 0 && r.Height > 0
}

// Display is one connected screen. WorkArea excludes the menu bar and Dock.
type Display struct {
	ID       string
	Main     bool
	Scale    float64
	Frame    Rect
	WorkArea Rect
}

// WindowFrame is the focused window as the engine observed it.
type WindowFrame struct {
	Frame        Rect
	DisplayID    string
	IsResizable  bool
	IsFullScreen bool
	IsMinimized  bool
}

// Edge names one side of a display work area.
type Edge string

const (
	EdgeLeft   Edge = "left"
	EdgeRight  Edge = "right"
	EdgeTop    Edge = "top"
	EdgeBottom Edge = "bottom"
)

// Edges lists the four sides in the order used by validation messages.
var Edges = []Edge{EdgeTop, EdgeRight, EdgeBottom, EdgeLeft}

func (e Edge) valid() bool {
	switch e {
	case EdgeLeft, EdgeRight, EdgeTop, EdgeBottom:
		return true
	}
	return false
}

// jsRound matches JavaScript Math.round: halves round toward positive infinity.
func jsRound(value float64) float64 {
	floor := math.Floor(value)
	if value-floor >= 0.5 {
		return floor + 1
	}
	return floor
}
