package geometry

import "math"

// ExactTolerancePoints is how far an observed frame may drift from a requested one and still count.
const ExactTolerancePoints = 2

// CycleTolerancePoints is how far a window may drift from its last applied frame and still count
// as "where the command put it" for cycling and restore history.
const CycleTolerancePoints = 2

// edgeTouchTolerance is the distance at which a requested zone counts as touching a work-area edge.
const edgeTouchTolerance = 1

// FramesMatch reports whether every edge and size of two frames agree within tolerance.
func FramesMatch(first, second Rect, tolerance float64) bool {
	return math.Abs(first.X-second.X) <= tolerance &&
		math.Abs(first.Y-second.Y) <= tolerance &&
		math.Abs(first.Width-second.Width) <= tolerance &&
		math.Abs(first.Height-second.Height) <= tolerance
}

// IsFrameInside reports whether a frame lies inside area, allowing the exact tolerance.
func IsFrameInside(frame, area Rect) bool {
	right := area.X + area.Width + ExactTolerancePoints
	bottom := area.Y + area.Height + ExactTolerancePoints
	return frame.X >= area.X-ExactTolerancePoints &&
		frame.Y >= area.Y-ExactTolerancePoints &&
		frame.X+frame.Width <= right &&
		frame.Y+frame.Height <= bottom
}

// clampOffsetInside keeps a frame's origin on one axis so the frame stays inside the extent.
func clampOffsetInside(offset, size, start, extent float64) float64 {
	return math.Max(start, math.Min(offset, start+extent-size))
}

// ClampFrameInside slides a frame back inside area without changing its size.
func ClampFrameInside(frame, area Rect) Rect {
	return Rect{
		X:      clampOffsetInside(frame.X, frame.Width, area.X, area.Width),
		Y:      clampOffsetInside(frame.Y, frame.Height, area.Y, area.Height),
		Width:  frame.Width,
		Height: frame.Height,
	}
}

// FitFrameInside shrinks a frame to the area size, then clamps it inside the area.
func FitFrameInside(frame, area Rect) Rect {
	frame.Width = math.Min(frame.Width, area.Width)
	frame.Height = math.Min(frame.Height, area.Height)
	return ClampFrameInside(frame, area)
}

// FitRequestedFrame returns the frame to request for a window. A resizable window takes the
// computed frame as is. A window that cannot resize keeps its size and is aligned inside the target
// zone: to the zone edge when the zone hugs a work-area edge, otherwise centered.
func FitRequestedFrame(computed Rect, window WindowFrame, area Rect) Rect {
	if window.IsResizable {
		return computed
	}
	return ClampFrameInside(Rect{
		X:      anchoredOrigin(computed.X, computed.Width, window.Frame.Width, area.X, area.Width),
		Y:      anchoredOrigin(computed.Y, computed.Height, window.Frame.Height, area.Y, area.Height),
		Width:  window.Frame.Width,
		Height: window.Frame.Height,
	}, area)
}

// anchoredOrigin places a fixed-size window along one axis of its zone.
func anchoredOrigin(zoneStart, zoneLength, length, areaStart, areaLength float64) float64 {
	touchesStart := zoneStart <= areaStart+edgeTouchTolerance
	touchesEnd := zoneStart+zoneLength >= areaStart+areaLength-edgeTouchTolerance
	anchor := 0.5
	if touchesStart != touchesEnd {
		anchor = 0
		if touchesEnd {
			anchor = 1
		}
	}
	return jsRound(zoneStart + (zoneLength-length)*anchor)
}
