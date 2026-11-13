package geometry

import (
	"fmt"
	"math"
)

// MaxGapPoints is the largest outer or inner gap the settings accept (window-management-types.ts).
const MaxGapPoints = 40

// CycleOrder is the sequence a repeated half command walks through.
type CycleOrder string

const (
	CycleHalfTwoThirdsThird CycleOrder = "half-two-thirds-third"
	CycleHalfThirdTwoThirds CycleOrder = "half-third-two-thirds"
)

// DisplayOrder decides which display counts as next or previous.
type DisplayOrder string

const (
	DisplayOrderLeftToRight DisplayOrder = "left-to-right"
	DisplayOrderTopToBottom DisplayOrder = "top-to-bottom"
)

// DisplayMoveSizing decides how a window keeps its size when it changes display.
type DisplayMoveSizing string

const (
	MoveProportional DisplayMoveSizing = "proportional"
	MoveKeepSize     DisplayMoveSizing = "keep-size"
)

// EdgeGaps is one gap per display edge, in points.
type EdgeGaps struct {
	Top, Right, Bottom, Left float64
}

// Edge returns the gap for one side.
func (g EdgeGaps) Edge(edge Edge) float64 {
	switch edge {
	case EdgeTop:
		return g.Top
	case EdgeRight:
		return g.Right
	case EdgeBottom:
		return g.Bottom
	case EdgeLeft:
		return g.Left
	}
	return 0
}

// Options are the window management settings the geometry consumes. Gaps are carried here as the
// engine input (outer, inner and apply-to-maximize), so there is no separate gaps type.
type Options struct {
	OuterGap           EdgeGaps
	InnerGap           float64
	ApplyGapToMaximize bool
	Cycle              bool
	CycleOrder         CycleOrder
	DisplayOrder       DisplayOrder
	DisplayMoveSizing  DisplayMoveSizing
}

// DefaultOptions returns the defaults from DEFAULT_WINDOW_MANAGEMENT_OPTIONS.
func DefaultOptions() Options {
	return Options{
		OuterGap:           EdgeGaps{Top: 8, Right: 8, Bottom: 8, Left: 8},
		InnerGap:           4,
		ApplyGapToMaximize: true,
		Cycle:              true,
		CycleOrder:         CycleHalfTwoThirdsThird,
		DisplayOrder:       DisplayOrderLeftToRight,
		DisplayMoveSizing:  MoveProportional,
	}
}

// UniformGaps returns options whose outer and inner gaps are all points (window-layout-gaps.fixture.ts).
func UniformGaps(points float64) Options {
	options := DefaultOptions()
	options.OuterGap = EdgeGaps{Top: points, Right: points, Bottom: points, Left: points}
	options.InnerGap = points
	options.ApplyGapToMaximize = true
	return options
}

const gapRangeText = "a whole number from 0 to 40"

var (
	innerGapMessage        = fmt.Sprintf("The inner gap must be %s.", gapRangeText)
	cycleOrderMessage      = "The cycle order must be half-two-thirds-third or half-third-two-thirds."
	displayOrderMessage    = "The display order must be left-to-right or top-to-bottom."
	displayMoveSizeMessage = "The display move sizing must be proportional or keep-size."
)

func outerGapMessage(edge Edge) string {
	return fmt.Sprintf("The outer %s gap must be %s.", edge, gapRangeText)
}

func isWholeGapPoints(value float64) bool {
	return value == math.Trunc(value) && value >= 0 && value <= MaxGapPoints
}

// DescribeInvalidOptions returns the palette message for the first invalid option, or nil.
// Messages match window-management-options-validation.ts.
func DescribeInvalidOptions(options Options) error {
	for _, edge := range Edges {
		if !isWholeGapPoints(options.OuterGap.Edge(edge)) {
			return copyError(outerGapMessage(edge))
		}
	}
	switch {
	case !isWholeGapPoints(options.InnerGap):
		return copyError(innerGapMessage)
	case options.CycleOrder != CycleHalfTwoThirdsThird && options.CycleOrder != CycleHalfThirdTwoThirds:
		return copyError(cycleOrderMessage)
	case options.DisplayOrder != DisplayOrderLeftToRight && options.DisplayOrder != DisplayOrderTopToBottom:
		return copyError(displayOrderMessage)
	case options.DisplayMoveSizing != MoveProportional && options.DisplayMoveSizing != MoveKeepSize:
		return copyError(displayMoveSizeMessage)
	}
	return nil
}
