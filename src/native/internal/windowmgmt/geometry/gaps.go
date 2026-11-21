package geometry

import "math"

// axisInsets are the gaps on the leading and trailing side of one axis.
type axisInsets struct {
	leading, trailing float64
}

// fractionInsetPair holds the insets for both axes of a unit-square fraction.
type fractionInsetPair struct {
	horizontal, vertical axisInsets
}

// unitRect is a fraction layout in unit-square coordinates.
type unitRect struct {
	x, y, width, height float64
}

// edgeEpsilon absorbs float error when deciding whether a fraction touches a display edge.
const edgeEpsilon = 1e-6

var noAxisInsets = axisInsets{}

func touchesLeadingEdge(start float64) bool {
	return start <= edgeEpsilon
}

func touchesTrailingEdge(start, length float64) bool {
	return start+length >= 1-edgeEpsilon
}

func coversWholeWorkArea(rect unitRect) bool {
	return touchesLeadingEdge(rect.x) && touchesTrailingEdge(rect.x, rect.width) &&
		touchesLeadingEdge(rect.y) && touchesTrailingEdge(rect.y, rect.height)
}

// interiorAxisInsets splits the inner gap so the leading half is the larger one for odd gaps.
func interiorAxisInsets(innerGap float64) axisInsets {
	return axisInsets{
		leading:  innerGap - math.Floor(innerGap/2),
		trailing: math.Floor(innerGap / 2),
	}
}

// fractionAxisInsets applies the outer gap where a fraction touches the display edge and half the
// inner gap where it abuts a neighbour.
func fractionAxisInsets(start, length float64, outer axisInsets, innerGap float64) axisInsets {
	interior := interiorAxisInsets(innerGap)
	insets := interior
	if touchesLeadingEdge(start) {
		insets.leading = outer.leading
	}
	if touchesTrailingEdge(start, length) {
		insets.trailing = outer.trailing
	}
	return insets
}

// insetsForFraction returns the gap insets for a fraction. A fraction that covers the whole work
// area gets no gaps unless the options apply gaps to maximize.
func insetsForFraction(rect unitRect, options Options) fractionInsetPair {
	if !options.ApplyGapToMaximize && coversWholeWorkArea(rect) {
		return fractionInsetPair{horizontal: noAxisInsets, vertical: noAxisInsets}
	}
	return fractionInsetPair{
		horizontal: fractionAxisInsets(rect.x, rect.width,
			axisInsets{leading: options.OuterGap.Left, trailing: options.OuterGap.Right}, options.InnerGap),
		vertical: fractionAxisInsets(rect.y, rect.height,
			axisInsets{leading: options.OuterGap.Top, trailing: options.OuterGap.Bottom}, options.InnerGap),
	}
}

// maximizedAxisInsets returns the outer gap for a maximize layout, or none when gaps do not apply
// to maximize.
func maximizedAxisInsets(options Options, leading, trailing float64) axisInsets {
	if options.ApplyGapToMaximize {
		return axisInsets{leading: leading, trailing: trailing}
	}
	return noAxisInsets
}
