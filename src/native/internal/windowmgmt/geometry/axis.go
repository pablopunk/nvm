package geometry

import "math"

// span is a one-dimensional extent: an origin and a length on one axis.
type span struct {
	origin, length float64
}

// axes splits a rectangle into its two spans so each axis is computed independently.
type axes struct {
	horizontal, vertical span
}

type edgeAlignment string

const (
	alignStart edgeAlignment = "start"
	alignEnd   edgeAlignment = "end"
	alignKeep  edgeAlignment = "keep"
)

// minResizedLength is the smallest length a resize step may shrink a window to.
const minResizedLength = 100

// edgeAlignments says, per edge, whether the window hugs the start, end or keeps its own
// position on each axis (EDGE_ALIGNMENTS in window-layouts.ts).
var edgeAlignments = map[Edge]struct{ horizontal, vertical edgeAlignment }{
	EdgeLeft:   {horizontal: alignStart, vertical: alignKeep},
	EdgeRight:  {horizontal: alignEnd, vertical: alignKeep},
	EdgeTop:    {horizontal: alignKeep, vertical: alignStart},
	EdgeBottom: {horizontal: alignKeep, vertical: alignEnd},
}

func axesOf(rect Rect) axes {
	return axes{
		horizontal: span{origin: rect.X, length: rect.Width},
		vertical:   span{origin: rect.Y, length: rect.Height},
	}
}

func frameFromAxes(a axes) Rect {
	return Rect{
		X:      jsRound(a.horizontal.origin),
		Y:      jsRound(a.vertical.origin),
		Width:  jsRound(a.horizontal.length),
		Height: jsRound(a.vertical.length),
	}
}

// placeSpan fits a span of the given length inside area, shrinking it when the area is shorter
// and sliding it back when the requested origin would leave the area.
func placeSpan(area span, length, origin float64) span {
	fitted := math.Min(length, area.length)
	latest := area.origin + area.length - fitted
	return span{origin: math.Min(math.Max(origin, area.origin), latest), length: fitted}
}

func centeredSpan(area span, length float64) span {
	return placeSpan(area, length, area.origin+(area.length-length)/2)
}

// clampedInsetSpan returns the part of area between origin and end, clamped to area.
func clampedInsetSpan(area span, origin, end float64) span {
	areaEnd := area.origin + area.length
	clampedOrigin := math.Min(math.Max(origin, area.origin), areaEnd)
	clampedEnd := math.Min(math.Max(end, clampedOrigin), areaEnd)
	return span{origin: clampedOrigin, length: clampedEnd - clampedOrigin}
}

func insetSpan(area span, insets axisInsets) span {
	return clampedInsetSpan(area, area.origin+insets.leading, area.origin+area.length-insets.trailing)
}

// fractionSpanInArea maps a unit interval onto area, rounding both edges so neighbours abut
// exactly, then applies the gap insets.
func fractionSpanInArea(area span, start, length float64, insets axisInsets) span {
	low := jsRound(area.origin + start*area.length)
	high := jsRound(area.origin + (start+length)*area.length)
	return clampedInsetSpan(area, low+insets.leading, high-insets.trailing)
}

func alignedSpan(area, current span, alignment edgeAlignment, gap float64) span {
	var origin float64
	switch alignment {
	case alignStart:
		origin = area.origin + gap
	case alignEnd:
		origin = area.origin + area.length - gap - current.length
	default:
		origin = current.origin
	}
	return placeSpan(area, current.length, origin)
}

// resizedSpan grows or shrinks a span by delta around its center. Shrinking never goes below
// minResizedLength, so a small window does not collapse; it is never enlarged by a shrink.
func resizedSpan(area, current span, delta float64) span {
	length := jsRound(math.Max(current.length+delta, math.Min(minResizedLength, current.length)))
	return placeSpan(area, length, current.origin+(current.length-length)/2)
}

func scaledLength(areaLength, fraction float64, maxLength *float64) float64 {
	limit := math.Inf(1)
	if maxLength != nil {
		limit = *maxLength
	}
	return jsRound(math.Min(fraction*areaLength, limit))
}
