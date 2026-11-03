package notes

import "github.com/pablopunk/nvm/native/internal/platform"

// FitFrame places a remembered floating-window frame on the displays that are connected now. The display is the
// one that holds the frame's center or, when none does, the one nearest to it. The frame is shrunk to that
// display's work area when it is larger, and moved so it lies inside the work area, as the 1.x window restore
// did. With no displays it returns the frame unchanged.
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

// nearestDisplay returns the display that contains p, or the one whose bounds are nearest to it. Ties keep the
// order of the displays.
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
