package geometry

// LayoutKind is the shape of a layout request, mirroring WindowLayoutSpec.kind.
type LayoutKind string

const (
	KindFraction         LayoutKind = "fraction"
	KindCenter           LayoutKind = "center"
	KindMaximizeWidth    LayoutKind = "maximize-width"
	KindMaximizeHeight   LayoutKind = "maximize-height"
	KindScaledCenter     LayoutKind = "scaled-center"
	KindEdge             LayoutKind = "edge"
	KindResizeStep       LayoutKind = "resize-step"
	KindDisplay          LayoutKind = "display"
	KindRestore          LayoutKind = "restore"
	KindToggleFullScreen LayoutKind = "toggle-full-screen"
	KindExact            LayoutKind = "exact"
)

// Direction is the direction of a resize step or a display move.
type Direction string

const (
	DirectionLarger   Direction = "larger"
	DirectionSmaller  Direction = "smaller"
	DirectionNext     Direction = "next"
	DirectionPrevious Direction = "previous"
)

// DimensionUnit is the unit of an exact value: absolute points or a percent of the work area.
type DimensionUnit string

const (
	UnitPoints  DimensionUnit = "points"
	UnitPercent DimensionUnit = "percent"
)

// Dimension is one exact x, y, width or height value.
type Dimension struct {
	Value float64
	Unit  DimensionUnit
}

// Layout is one layout request. Only the fields for its Kind are meaningful:
//   - fraction: X, Y, Width, Height (unit fractions) and an optional CycleGroup.
//   - scaled-center: WidthFraction, HeightFraction and optional MaxWidth or MaxHeight caps.
//   - edge: Edge. resize-step: Direction and Step (fraction of the work area).
//   - display: Direction (next or previous). exact: the Exact* dimensions that are set.
//
// Optional numbers are pointers so "unset" and "zero" stay distinct, as in the TypeScript.
type Layout struct {
	Kind LayoutKind

	X, Y, Width, Height float64
	CycleGroup          Edge

	WidthFraction, HeightFraction float64
	MaxWidth, MaxHeight           *float64

	Edge      Edge
	Direction Direction
	Step      float64

	ExactX, ExactY, ExactWidth, ExactHeight *Dimension
}

// IsServiceHandled reports whether a kind needs the engine rather than frame arithmetic.
func IsServiceHandled(kind LayoutKind) bool {
	return kind == KindRestore || kind == KindDisplay || kind == KindToggleFullScreen
}

// fractionLayout builds a fraction layout, optionally tagged with a cycle group.
func fractionLayout(x, y, width, height float64, group Edge) Layout {
	return Layout{Kind: KindFraction, X: x, Y: y, Width: width, Height: height, CycleGroup: group}
}

func plainFraction(x, y, width, height float64) Layout {
	return fractionLayout(x, y, width, height, "")
}

func scaledCenter(widthFraction, heightFraction float64) Layout {
	return Layout{Kind: KindScaledCenter, WidthFraction: widthFraction, HeightFraction: heightFraction}
}

func edgeMove(edge Edge) Layout {
	return Layout{Kind: KindEdge, Edge: edge}
}

func resizeStep(direction Direction, step float64) Layout {
	return Layout{Kind: KindResizeStep, Direction: direction, Step: step}
}

func displayMove(direction Direction) Layout {
	return Layout{Kind: KindDisplay, Direction: direction}
}

// Exact layouts with unset fields keep the window's current value for that field.
func exactLayout(x, y, width, height *Dimension) Layout {
	return Layout{Kind: KindExact, ExactX: x, ExactY: y, ExactWidth: width, ExactHeight: height}
}
