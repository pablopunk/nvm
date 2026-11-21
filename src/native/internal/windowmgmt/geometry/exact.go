package geometry

import (
	"math"
)

// ExactField names one of the four exact layout fields. Offsets are x and y; sizes are the rest.
type ExactField string

const (
	FieldX      ExactField = "x"
	FieldY      ExactField = "y"
	FieldWidth  ExactField = "width"
	FieldHeight ExactField = "height"
)

// exactFields is the canonical order used for validation and error reporting.
var exactFields = []ExactField{FieldX, FieldY, FieldWidth, FieldHeight}

type exactKind string

const (
	kindOffset exactKind = "offset"
	kindSize   exactKind = "size"
)

const (
	maxExactPoints  = 100_000
	maxExactPercent = 100
	percentScale    = 100
	minResizedPts   = 1
)

var exactKindOf = map[ExactField]exactKind{
	FieldX: kindOffset, FieldY: kindOffset, FieldWidth: kindSize, FieldHeight: kindSize,
}

func maxExactValue(unit DimensionUnit) float64 {
	if unit == UnitPercent {
		return maxExactPercent
	}
	return maxExactPoints
}

func dimensionInPoints(dimension Dimension, extent float64) float64 {
	if dimension.Unit == UnitPercent {
		return (dimension.Value / percentScale) * extent
	}
	return dimension.Value
}

func exactOriginOnAxis(dimension *Dimension, workAreaOrigin, workAreaExtent, currentOrigin float64) float64 {
	if dimension == nil {
		return currentOrigin
	}
	return workAreaOrigin + jsRound(dimensionInPoints(*dimension, workAreaExtent))
}

func exactSizeOnAxis(dimension *Dimension, workAreaExtent, currentSize float64) float64 {
	if dimension == nil {
		return currentSize
	}
	return math.Max(minResizedPts, jsRound(dimensionInPoints(*dimension, workAreaExtent)))
}

// exactFrame places and sizes a window from its exact fields. Unset fields keep the current value,
// and the result is fitted and clamped inside the work area.
func exactFrame(layout Layout, workArea, current Rect) Rect {
	return FitFrameInside(Rect{
		X:      exactOriginOnAxis(layout.ExactX, workArea.X, workArea.Width, current.X),
		Y:      exactOriginOnAxis(layout.ExactY, workArea.Y, workArea.Height, current.Y),
		Width:  exactSizeOnAxis(layout.ExactWidth, workArea.Width, current.Width),
		Height: exactSizeOnAxis(layout.ExactHeight, workArea.Height, current.Height),
	}, workArea)
}

// exactDimension returns the dimension set for one field, or nil when the field is unset.
func exactDimension(layout Layout, field ExactField) *Dimension {
	switch field {
	case FieldX:
		return layout.ExactX
	case FieldY:
		return layout.ExactY
	case FieldWidth:
		return layout.ExactWidth
	}
	return layout.ExactHeight
}

func exactRangeCopy(field ExactField, kind exactKind, unit DimensionUnit) string {
	limit := formatNumber(maxExactValue(unit))
	suffix := " points"
	if unit == UnitPercent {
		suffix = " percent"
	}
	if kind == kindOffset {
		return "The window " + string(field) + " must be between 0 and " + limit + suffix + "."
	}
	return "The window " + string(field) + " must be above 0 and at most " + limit + suffix + "."
}

func describeInvalidExactDimension(field ExactField, dimension Dimension) string {
	if math.IsNaN(dimension.Value) || math.IsInf(dimension.Value, 0) {
		return "The window " + string(field) + " must be a finite number."
	}
	if dimension.Unit != UnitPoints && dimension.Unit != UnitPercent {
		return "The window " + string(field) + " unit must be points or percent."
	}
	kind := exactKindOf[field]
	aboveMinimum := dimension.Value >= 0
	if kind == kindSize {
		aboveMinimum = dimension.Value > 0
	}
	if aboveMinimum && dimension.Value <= maxExactValue(dimension.Unit) {
		return ""
	}
	return exactRangeCopy(field, kind, dimension.Unit)
}

// DescribeInvalidExactLayout returns the first invalid exact field as an error, or nil. Messages
// match window-management-exact-validation.ts.
func DescribeInvalidExactLayout(layout Layout) error {
	present := false
	for _, field := range exactFields {
		dimension := exactDimension(layout, field)
		if dimension == nil {
			continue
		}
		present = true
		if message := describeInvalidExactDimension(field, *dimension); message != "" {
			return copyError(message)
		}
	}
	if !present {
		return copyError(MessageNoFieldsExact)
	}
	return nil
}
