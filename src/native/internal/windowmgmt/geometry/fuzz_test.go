package geometry

import (
	"math"
	"testing"
)

// Seeded fuzz tests: hostile numbers, unknown kinds and degenerate displays must never panic, and a
// request that passes validation must resolve to finite numbers.

const fuzzIterations = 4000

func finiteRect(frame Rect) bool {
	for _, value := range []float64{frame.X, frame.Y, frame.Width, frame.Height} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func TestFuzzHostileRequestsNeverPanicOrYieldNaN(t *testing.T) {
	rng := newPropertyRand(100)
	for iteration := 0; iteration < fuzzIterations; iteration++ {
		layout := randomHostileLayout(rng)
		options := randomHostileOptions(rng)
		display := Display{ID: "fuzz", Scale: 1, Frame: randomFiniteRect(rng), WorkArea: randomFiniteRect(rng)}
		window := WindowFrame{Frame: randomFiniteRect(rng), DisplayID: "fuzz", IsResizable: rng.IntN(2) == 0}

		valid := DescribeInvalidRequest(layout, options) == nil
		frame, _, err := Resolve(layout, display, window, options, nil)
		if valid && err == nil && display.WorkArea.HasArea() && !finiteRect(frame) {
			t.Fatalf("iteration %d: valid layout %+v resolved to %+v", iteration, layout, frame)
		}

		displays := []Display{display, randomDisplay(rng, "other")}
		if _, err := PickDisplayForFrame(displays, window.Frame); err != nil {
			t.Fatalf("iteration %d: PickDisplayForFrame: %v", iteration, err)
		}
		moved, _, moveErr := MoveWindowToAdjacentDisplay(displays, window, layout.Direction, options)
		if moveErr == nil && !finiteRect(moved) {
			t.Fatalf("iteration %d: display move produced a non-finite frame", iteration)
		}
		_, _ = AdjacentDisplay(displays, display, layout.Direction, options.DisplayOrder)
		_, _ = CycleSequence(layout.CycleGroup, options.CycleOrder)
		_, _, _ = ResolveCycleStep(layout, nil, window, options)
		_ = Verify(window.Frame, display.WorkArea, window.Frame, Size{Width: layout.Step, Height: layout.X})
		_ = RememberApplied(nil, window, window.Frame, layout.Step > 0, CycleRecord{Group: layout.CycleGroup, Index: 1})
		_ = FitRequestedFrame(window.Frame, window, display.WorkArea)
		_ = NeedsOverflowCorrection(Partial, window.Frame, display.WorkArea)
	}
}

func TestFuzzNeighborSearchNeverPanics(t *testing.T) {
	rng := newPropertyRand(101)
	for iteration := 0; iteration < fuzzIterations/4; iteration++ {
		origin, candidates := randomNeighborField(rng, rng.IntN(12))
		for _, direction := range []NeighborDirection{NeighborLeft, NeighborRight, NeighborUp, NeighborDown, "diagonal"} {
			_, _ = NearestWindowInDirection(origin, candidates, direction, NeighborOptions{Wrap: iteration%2 == 0})
		}
	}
}

func TestFuzzMemoryStaysBoundedUnderChurn(t *testing.T) {
	rng := newPropertyRand(102)
	memory := NewMemory()
	for iteration := 0; iteration < fuzzIterations; iteration++ {
		key := []string{"1:1", "1:2", "2:1", "2:2", "3:9"}[rng.IntN(5)]
		switch rng.IntN(4) {
		case 0:
			memory.Write(key, History{})
		case 1:
			memory.Forget(key)
		case 2:
			memory.DropProcess(1 + rng.IntN(3))
		default:
			memory.Write(string(rune('a'+rng.IntN(26)))+":"+key, History{})
		}
		if memory.Len() > MaxRememberedWindows {
			t.Fatalf("iteration %d: memory holds %d windows, cap %d", iteration, memory.Len(), MaxRememberedWindows)
		}
		if len(memory.order) != memory.Len() {
			t.Fatalf("iteration %d: order %d and entries %d disagree", iteration, len(memory.order), memory.Len())
		}
	}
}
