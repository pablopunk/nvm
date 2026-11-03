package geometry

import "math"

// Resize step bounds, as a fraction of the work area (window-management-validation.ts).
const (
	minResizeStep = 0.01
	maxResizeStep = 0.25
	fractionSlack = 1e-6
)

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func unitFraction(value float64) bool {
	return finite(value) && value > 0 && value <= 1
}

func optionalPositiveLength(value *float64) bool {
	return value == nil || (finite(*value) && *value > 0)
}

func validFractionRect(layout Layout) bool {
	x, y, width, height := layout.X, layout.Y, layout.Width, layout.Height
	return finite(x) && finite(y) && finite(width) && finite(height) &&
		x >= 0 && y >= 0 && x <= 1 && y <= 1 &&
		width > 0 && width <= 1 && height > 0 && height <= 1 &&
		x+width <= 1+fractionSlack && y+height <= 1+fractionSlack
}

func validFraction(layout Layout) bool {
	return validFractionRect(layout) && (layout.CycleGroup == "" || layout.CycleGroup.valid())
}

func validResizeStep(layout Layout) bool {
	if layout.Direction != DirectionLarger && layout.Direction != DirectionSmaller {
		return false
	}
	return finite(layout.Step) && layout.Step >= minResizeStep && layout.Step <= maxResizeStep
}

func validDisplayMove(layout Layout) bool {
	return layout.Direction == DirectionNext || layout.Direction == DirectionPrevious
}

// DescribeInvalidLayout validates a layout request on its own. Exact layouts report the specific
// field; every other invalid shape reports the generic invalid-spec error.
func DescribeInvalidLayout(layout Layout) error {
	valid := true
	switch layout.Kind {
	case KindExact:
		return DescribeInvalidExactLayout(layout)
	case KindFraction:
		valid = validFraction(layout)
	case KindScaledCenter:
		valid = unitFraction(layout.WidthFraction) && unitFraction(layout.HeightFraction) &&
			optionalPositiveLength(layout.MaxWidth) && optionalPositiveLength(layout.MaxHeight)
	case KindEdge:
		valid = layout.Edge.valid()
	case KindResizeStep:
		valid = validResizeStep(layout)
	case KindDisplay:
		valid = validDisplayMove(layout)
	case KindCenter, KindMaximizeWidth, KindMaximizeHeight, KindRestore, KindToggleFullScreen:
		valid = true
	default:
		valid = false
	}
	if !valid {
		return ErrInvalidSpec
	}
	return nil
}

// DescribeInvalidRequest validates options first, then the layout, as the engine does before it
// touches any window. It returns the palette message for the first problem, or nil.
func DescribeInvalidRequest(layout Layout, options Options) error {
	if err := DescribeInvalidOptions(options); err != nil {
		return err
	}
	return DescribeInvalidLayout(layout)
}
