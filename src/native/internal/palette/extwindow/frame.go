package extwindow

import "github.com/pablopunk/nvm/native/internal/platform"

// Frame is a window's place: its top left corner and its size, in points.
type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// placementMargin is the distance top-center and bottom-center windows keep from the edge of the work area.
const placementMargin = 48

// FitFrame places a remembered frame on the displays connected now. The display is the one that holds the frame's
// center, or the nearest one; the frame shrinks to its work area and moves inside it. With no displays it returns the
// frame unchanged.
func FitFrame(saved Frame, displays []platform.Display) Frame {
	if len(displays) == 0 {
		return saved
	}
	center := platform.Point{X: saved.X + saved.Width/2, Y: saved.Y + saved.Height/2}
	area := nearestDisplay(center, displays).WorkArea
	width := min(max(saved.Width, 1), area.Width)
	height := min(max(saved.Height, 1), area.Height)
	return Frame{
		X:      clampSpan(saved.X, area.X, area.Width, width),
		Y:      clampSpan(saved.Y, area.Y, area.Height, height),
		Width:  width,
		Height: height,
	}
}

// PlaceFrame puts a window of width by height on area at position: center (the default), top-center or
// bottom-center. The window is shrunk to area when it is larger.
func PlaceFrame(area platform.Rect, width, height int, position string) Frame {
	width = min(width, area.Width)
	height = min(height, area.Height)
	frame := Frame{X: area.X + (area.Width-width)/2, Y: area.Y + (area.Height-height)/2, Width: width, Height: height}
	switch position {
	case "top-center":
		frame.Y = area.Y + placementMargin
	case "bottom-center":
		frame.Y = area.Y + area.Height - height - placementMargin
	}
	return frame
}

// nearestDisplay returns the display that contains p, or the one whose bounds are nearest to it.
func nearestDisplay(p platform.Point, displays []platform.Display) platform.Display {
	best, bestDistance := displays[0], -1
	for _, display := range displays {
		if display.Bounds.Contains(p) {
			return display
		}
		if d := distanceToRect(p, display.Bounds); bestDistance < 0 || d < bestDistance {
			best, bestDistance = display, d
		}
	}
	return best
}

// distanceToRect is the squared distance from p to the nearest point of r; it is zero inside r.
func distanceToRect(p platform.Point, r platform.Rect) int {
	dx := max(r.X-p.X, 0, p.X-(r.X+r.Width-1))
	dy := max(r.Y-p.Y, 0, p.Y-(r.Y+r.Height-1))
	return dx*dx + dy*dy
}

// clampSpan moves an origin so a span of size lies inside [origin, origin+available).
func clampSpan(value, origin, available, size int) int {
	return min(max(value, origin), max(origin, origin+available-size))
}
