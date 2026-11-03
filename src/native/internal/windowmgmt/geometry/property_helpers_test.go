package geometry

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Deterministic random generators for the property and fuzz tests. Every test uses a fixed seed, so
// failures reproduce; there is no wall clock and no goroutine.

const propertyIterations = 300

func newPropertyRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// randomDisplay returns a display with integer geometry. Origins can be negative, and the work
// area is inset by a menu bar and Dock the way real displays are.
func randomDisplay(rng *rand.Rand, id string) Display {
	width := float64(800 + rng.IntN(3041))
	height := float64(600 + rng.IntN(1561))
	x := float64(rng.IntN(8001) - 4000)
	y := float64(rng.IntN(8001) - 4000)
	insetTop := []float64{0, 25}[rng.IntN(2)]
	insetBottom := []float64{0, 70}[rng.IntN(2)]
	frame := Rect{X: x, Y: y, Width: width, Height: height}
	workArea := Rect{X: x, Y: y + insetTop, Width: width, Height: height - insetTop - insetBottom}
	return Display{ID: id, Scale: []float64{1, 1.5, 2}[rng.IntN(3)], Frame: frame, WorkArea: workArea}
}

// randomGaps returns uniform-or-per-edge gaps in whole points, within the settings range.
func randomGaps(rng *rand.Rand) Options {
	options := DefaultOptions()
	options.OuterGap = EdgeGaps{
		Top:    float64(rng.IntN(MaxGapPoints + 1)),
		Right:  float64(rng.IntN(MaxGapPoints + 1)),
		Bottom: float64(rng.IntN(MaxGapPoints + 1)),
		Left:   float64(rng.IntN(MaxGapPoints + 1)),
	}
	options.InnerGap = float64(rng.IntN(MaxGapPoints + 1))
	options.ApplyGapToMaximize = rng.IntN(2) == 0
	return options
}

// randomWindow returns a window that may sit on or off the display, may be oversized, and may or
// may not be resizable.
func randomWindow(rng *rand.Rand, display Display) WindowFrame {
	area := display.WorkArea
	width := float64(1 + rng.IntN(int(area.Width)+int(area.Width)/2))
	height := float64(1 + rng.IntN(int(area.Height)+int(area.Height)/2))
	x := area.X - area.Width/2 + float64(rng.IntN(int(area.Width)*2))
	y := area.Y - area.Height/2 + float64(rng.IntN(int(area.Height)*2))
	return WindowFrame{
		Frame:       Rect{X: x, Y: y, Width: width, Height: height},
		DisplayID:   display.ID,
		IsResizable: rng.IntN(4) != 0,
	}
}

// fitsInside reports whether a window's size fits in the work area, the precondition for a
// non-resizable window to be placed fully inside it.
func fitsInside(window Rect, area Rect) bool {
	return window.Width <= area.Width && window.Height <= area.Height
}

// randomFramePresetIDs returns a catalog preset that resolves to a frame (not service-handled).
func randomFramePresetIDs() []PresetID {
	var ids []PresetID
	for _, id := range PresetIDs() {
		if layout := mustPresetForProperties(id); !IsServiceHandled(layout.Kind) {
			ids = append(ids, id)
		}
	}
	return ids
}

func mustPresetForProperties(id PresetID) Layout {
	layout, ok := Preset(id)
	if !ok {
		panic(fmt.Sprintf("preset %q missing", id))
	}
	return layout
}

// randomExactLayout returns a valid exact layout with a random subset of set fields.
func randomExactLayout(rng *rand.Rand) Layout {
	dimension := func(offset bool) *Dimension {
		if rng.IntN(3) == 0 {
			return nil
		}
		if rng.IntN(2) == 0 {
			value := float64(rng.IntN(maxExactPoints + 1))
			if !offset && value == 0 {
				value = 1
			}
			return &Dimension{Value: value, Unit: UnitPoints}
		}
		value := rng.Float64() * maxExactPercent
		if !offset && value == 0 {
			value = 1
		}
		return &Dimension{Value: value, Unit: UnitPercent}
	}
	layout := Layout{Kind: KindExact, ExactX: dimension(true), ExactY: dimension(true), ExactWidth: dimension(false), ExactHeight: dimension(false)}
	if layout.ExactX == nil && layout.ExactY == nil && layout.ExactWidth == nil && layout.ExactHeight == nil {
		layout.ExactWidth = &Dimension{Value: 100, Unit: UnitPoints}
	}
	return layout
}

// randomFloat returns a value from a pool that deliberately includes the hostile numbers.
func randomFloat(rng *rand.Rand) float64 {
	pool := []float64{0, 1, -1, 0.5, -0.5, 40, 41, 1e9, -1e9, 1e-9, math.NaN(), math.Inf(1), math.Inf(-1)}
	if rng.IntN(3) == 0 {
		return pool[rng.IntN(len(pool))]
	}
	return rng.Float64()*4 - 2
}

// randomFiniteRect returns a finite rectangle that may have zero or negative size, for hostile inputs.
func randomFiniteRect(rng *rand.Rand) Rect {
	return Rect{
		X:      float64(rng.IntN(4001) - 2000),
		Y:      float64(rng.IntN(4001) - 2000),
		Width:  float64(rng.IntN(3001) - 100),
		Height: float64(rng.IntN(3001) - 100),
	}
}

// randomHostileLayout returns a layout built from hostile numbers and kinds, so it may be invalid.
func randomHostileLayout(rng *rand.Rand) Layout {
	kinds := []LayoutKind{KindFraction, KindCenter, KindMaximizeWidth, KindMaximizeHeight, KindScaledCenter,
		KindEdge, KindResizeStep, KindDisplay, KindRestore, KindToggleFullScreen, KindExact, "bogus"}
	edges := []Edge{EdgeLeft, EdgeRight, EdgeTop, EdgeBottom, "diagonal", ""}
	directions := []Direction{DirectionLarger, DirectionSmaller, DirectionNext, DirectionPrevious, "sideways"}
	dimension := func() *Dimension {
		if rng.IntN(2) == 0 {
			return nil
		}
		unit := UnitPoints
		if rng.IntN(2) == 0 {
			unit = UnitPercent
		}
		return &Dimension{Value: randomFloat(rng), Unit: unit}
	}
	maxLength := func() *float64 {
		if rng.IntN(2) == 0 {
			return nil
		}
		value := randomFloat(rng)
		return &value
	}
	return Layout{
		Kind: kinds[rng.IntN(len(kinds))],
		X:    randomFloat(rng), Y: randomFloat(rng), Width: randomFloat(rng), Height: randomFloat(rng),
		CycleGroup:    edges[rng.IntN(len(edges))],
		WidthFraction: randomFloat(rng), HeightFraction: randomFloat(rng),
		MaxWidth: maxLength(), MaxHeight: maxLength(),
		Edge:      edges[rng.IntN(len(edges))],
		Direction: directions[rng.IntN(len(directions))],
		Step:      randomFloat(rng),
		ExactX:    dimension(), ExactY: dimension(), ExactWidth: dimension(), ExactHeight: dimension(),
	}
}

// randomHostileOptions returns options whose numbers may fall outside the settings range.
func randomHostileOptions(rng *rand.Rand) Options {
	options := DefaultOptions()
	options.OuterGap = EdgeGaps{Top: randomFloat(rng), Right: randomFloat(rng), Bottom: randomFloat(rng), Left: randomFloat(rng)}
	options.InnerGap = randomFloat(rng)
	options.ApplyGapToMaximize = rng.IntN(2) == 0
	options.Cycle = rng.IntN(2) == 0
	if rng.IntN(4) == 0 {
		options.CycleOrder = "third-first"
	}
	return options
}
