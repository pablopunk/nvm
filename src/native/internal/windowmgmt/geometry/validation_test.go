package geometry

import (
	"math"
	"testing"
)

// Ported from 1.x/electron/os/window-management-validation.test.ts (layout and option cases),
// window-management-exact-validation.test.ts, and the option validation parts of
// window-management-options-validation.ts. Cases the TypeScript expresses only with undefined or a
// non-boolean value are not representable in the typed Go model and are listed in the report.

func validOptions() Options { return DefaultOptions() }

func pointsValue(value float64) Dimension { return Dimension{Value: value, Unit: UnitPoints} }

func percentValue(value float64) Dimension { return Dimension{Value: value, Unit: UnitPercent} }

func exactWith(field ExactField, dimension Dimension) Layout {
	layout := Layout{Kind: KindExact}
	switch field {
	case FieldX:
		layout.ExactX = &dimension
	case FieldY:
		layout.ExactY = &dimension
	case FieldWidth:
		layout.ExactWidth = &dimension
	default:
		layout.ExactHeight = &dimension
	}
	return layout
}

func floatPtr(value float64) *float64 { return &value }

const noFieldsCopy = "Choose at least one window value: x, y, width, or height."

type layoutCase struct {
	name    string
	layout  Layout
	message string
}

var rejectedExactSpecs = []layoutCase{
	{"no fields", Layout{Kind: KindExact}, noFieldsCopy},
	{"a non-finite x value", exactWith(FieldX, pointsValue(math.NaN())), "The window x must be a finite number."},
	{"an infinite width value", exactWith(FieldWidth, pointsValue(math.Inf(1))), "The window width must be a finite number."},
	{"an unknown unit", exactWith(FieldHeight, Dimension{Value: 10, Unit: "pixels"}), "The window height unit must be points or percent."},
	{"a negative point offset", exactWith(FieldX, pointsValue(-1)), "The window x must be between 0 and 100000 points."},
	{"a point offset above 100000", exactWith(FieldY, pointsValue(100_001)), "The window y must be between 0 and 100000 points."},
	{"a percent offset above 100", exactWith(FieldX, percentValue(101)), "The window x must be between 0 and 100 percent."},
	{"a zero point width", exactWith(FieldWidth, pointsValue(0)), "The window width must be above 0 and at most 100000 points."},
	{"a point height above 100000", exactWith(FieldHeight, pointsValue(100_001)), "The window height must be above 0 and at most 100000 points."},
	{"a zero percent height", exactWith(FieldHeight, percentValue(0)), "The window height must be above 0 and at most 100 percent."},
	{"a percent width above 100", exactWith(FieldWidth, percentValue(100.5)), "The window width must be above 0 and at most 100 percent."},
}

var acceptedExactSpecs = []struct {
	name   string
	layout Layout
}{
	{"only x in points", exactWith(FieldX, pointsValue(0))},
	{"only y in percent at the offset maximum", exactWith(FieldY, percentValue(100))},
	{"a point width at the size maximum", exactWith(FieldWidth, pointsValue(100_000))},
	{"every field in mixed units", Layout{
		Kind:        KindExact,
		ExactX:      dimPtr(pointsValue(10)),
		ExactY:      dimPtr(percentValue(10)),
		ExactWidth:  dimPtr(percentValue(50)),
		ExactHeight: dimPtr(pointsValue(300)),
	}},
}

func dimPtr(dimension Dimension) *Dimension { return &dimension }

func TestExactLayoutRejectionsPort(t *testing.T) {
	for _, c := range rejectedExactSpecs {
		t.Run("rejects an exact layout with "+c.name+" and says why", func(t *testing.T) {
			err := DescribeInvalidRequest(c.layout, validOptions())
			if err == nil || err.Error() != c.message {
				t.Fatalf("error = %v, want %q", err, c.message)
			}
		})
	}
}

func TestExactLayoutAcceptancePort(t *testing.T) {
	for _, c := range acceptedExactSpecs {
		t.Run("accepts an exact layout with "+c.name, func(t *testing.T) {
			if err := DescribeInvalidRequest(c.layout, validOptions()); err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}

var invalidLayoutSpecs = []layoutCase{
	{"scaled-center width of zero", Layout{Kind: KindScaledCenter, WidthFraction: 0, HeightFraction: 0.9}, MessageInvalidSpec},
	{"scaled-center width above one", Layout{Kind: KindScaledCenter, WidthFraction: 1.5, HeightFraction: 0.9}, MessageInvalidSpec},
	{"scaled-center NaN height", Layout{Kind: KindScaledCenter, WidthFraction: 0.9, HeightFraction: math.NaN()}, MessageInvalidSpec},
	{"scaled-center zero maxWidth", Layout{Kind: KindScaledCenter, WidthFraction: 0.6, HeightFraction: 0.6, MaxWidth: floatPtr(0)}, MessageInvalidSpec},
	{"scaled-center infinite maxHeight", Layout{Kind: KindScaledCenter, WidthFraction: 0.6, HeightFraction: 0.6, MaxHeight: floatPtr(math.Inf(1))}, MessageInvalidSpec},
	{"edge outside the four sides", Layout{Kind: KindEdge, Edge: "diagonal"}, MessageInvalidSpec},
	{"resize-step below the minimum", Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: 0.005}, MessageInvalidSpec},
	{"resize-step above the maximum", Layout{Kind: KindResizeStep, Direction: DirectionSmaller, Step: 0.3}, MessageInvalidSpec},
	{"resize-step NaN", Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: math.NaN()}, MessageInvalidSpec},
	{"resize-step unknown direction", Layout{Kind: KindResizeStep, Direction: "bigger", Step: 0.05}, MessageInvalidSpec},
	{"display unknown direction", Layout{Kind: KindDisplay, Direction: "sideways"}, MessageInvalidSpec},
	{"fraction unknown cycle group", Layout{Kind: KindFraction, X: 0, Y: 0, Width: 0.5, Height: 1, CycleGroup: "middle"}, MessageInvalidSpec},
	{"unknown kind", Layout{Kind: "bogus"}, MessageInvalidSpec},
}

var validLayoutSpecs = []struct {
	name   string
	layout Layout
}{
	{"center", Layout{Kind: KindCenter}},
	{"maximize-width", Layout{Kind: KindMaximizeWidth}},
	{"maximize-height", Layout{Kind: KindMaximizeHeight}},
	{"restore", Layout{Kind: KindRestore}},
	{"toggle-full-screen", Layout{Kind: KindToggleFullScreen}},
	{"scaled-center without caps", Layout{Kind: KindScaledCenter, WidthFraction: 0.9, HeightFraction: 0.9}},
	{"scaled-center with caps", Layout{Kind: KindScaledCenter, WidthFraction: 0.6, HeightFraction: 0.6, MaxWidth: floatPtr(1025), MaxHeight: floatPtr(900)}},
	{"edge left", Layout{Kind: KindEdge, Edge: EdgeLeft}},
	{"edge right", Layout{Kind: KindEdge, Edge: EdgeRight}},
	{"edge top", Layout{Kind: KindEdge, Edge: EdgeTop}},
	{"edge bottom", Layout{Kind: KindEdge, Edge: EdgeBottom}},
	{"resize-step larger at the minimum", Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: 0.01}},
	{"resize-step smaller at the maximum", Layout{Kind: KindResizeStep, Direction: DirectionSmaller, Step: 0.25}},
	{"display next", Layout{Kind: KindDisplay, Direction: DirectionNext}},
	{"display previous", Layout{Kind: KindDisplay, Direction: DirectionPrevious}},
	{"fraction with a cycle group", Layout{Kind: KindFraction, X: 0, Y: 0, Width: 1, Height: 0.5, CycleGroup: EdgeTop}},
}

func TestInvalidLayoutSpecsPort(t *testing.T) {
	for _, c := range invalidLayoutSpecs {
		t.Run("rejects an invalid "+c.name+" with the spec message", func(t *testing.T) {
			err := DescribeInvalidRequest(c.layout, validOptions())
			if err == nil || err.Error() != c.message {
				t.Fatalf("error = %v, want %q", err, c.message)
			}
		})
	}
}

func TestValidLayoutSpecsPort(t *testing.T) {
	for _, c := range validLayoutSpecs {
		t.Run("accepts a valid "+c.name, func(t *testing.T) {
			if err := DescribeInvalidRequest(c.layout, validOptions()); err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}

type optionCase struct {
	name    string
	mutate  func(*Options)
	message string
}

const (
	innerGapRangeMessage = "The inner gap must be a whole number from 0 to 40."
	cycleOrderCopy       = "The cycle order must be half-two-thirds-third or half-third-two-thirds."
	displayOrderCopy     = "The display order must be left-to-right or top-to-bottom."
	displaySizingCopy    = "The display move sizing must be proportional or keep-size."
)

func outerGapRangeMessage(edge Edge) string {
	return "The outer " + string(edge) + " gap must be a whole number from 0 to 40."
}

var invalidOptionCases = []optionCase{
	{"an inner gap above 40", func(o *Options) { o.InnerGap = 41 }, innerGapRangeMessage},
	{"a fractional inner gap", func(o *Options) { o.InnerGap = 1.5 }, innerGapRangeMessage},
	{"a negative inner gap", func(o *Options) { o.InnerGap = -1 }, innerGapRangeMessage},
	{"an outer right gap above 40", func(o *Options) { o.OuterGap.Right = 41 }, outerGapRangeMessage(EdgeRight)},
	{"a negative outer bottom gap", func(o *Options) { o.OuterGap.Bottom = -2 }, outerGapRangeMessage(EdgeBottom)},
	{"an unknown cycle order", func(o *Options) { o.CycleOrder = "third-first" }, cycleOrderCopy},
	{"an unknown display order", func(o *Options) { o.DisplayOrder = "diagonal" }, displayOrderCopy},
	{"an unknown display move sizing", func(o *Options) { o.DisplayMoveSizing = "stretch" }, displaySizingCopy},
}

func TestInvalidOptionCasesPort(t *testing.T) {
	for _, c := range invalidOptionCases {
		t.Run("rejects "+c.name+" with the option message", func(t *testing.T) {
			options := validOptions()
			c.mutate(&options)
			err := DescribeInvalidRequest(Layout{Kind: KindCenter}, options)
			if err == nil || err.Error() != c.message {
				t.Fatalf("error = %v, want %q", err, c.message)
			}
		})
	}
}

func TestOptionLimitsAndCycleOffPort(t *testing.T) {
	t.Run("accepts gaps at the 0 and 40 point limits on every edge", func(t *testing.T) {
		options := validOptions()
		options.OuterGap = EdgeGaps{Top: 0, Right: 40, Bottom: 0, Left: 40}
		options.InnerGap = 40
		if err := DescribeInvalidRequest(Layout{Kind: KindCenter}, options); err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
	})
	t.Run("accepts cycle set to false", func(t *testing.T) {
		options := validOptions()
		options.Cycle = false
		if err := DescribeInvalidRequest(Layout{Kind: KindCenter}, options); err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
	})
	t.Run("accepts every cycle order, display order and display move sizing", func(t *testing.T) {
		for _, cycleOrder := range []CycleOrder{CycleHalfTwoThirdsThird, CycleHalfThirdTwoThirds} {
			for _, displayOrder := range []DisplayOrder{DisplayOrderLeftToRight, DisplayOrderTopToBottom} {
				for _, sizing := range []DisplayMoveSizing{MoveProportional, MoveKeepSize} {
					options := validOptions()
					options.CycleOrder = cycleOrder
					options.DisplayOrder = displayOrder
					options.DisplayMoveSizing = sizing
					if err := DescribeInvalidRequest(Layout{Kind: KindCenter}, options); err != nil {
						t.Fatalf("%s/%s/%s: %v", cycleOrder, displayOrder, sizing, err)
					}
				}
			}
		}
	})
}
