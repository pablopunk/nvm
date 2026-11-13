package geometry

import (
	"fmt"
	"math"
	"testing"
)

// Property tests over random displays (negative origins, menu bar and Dock insets, varying sizes)
// for every frame-producing preset and for random exact layouts.

func TestPropertyEveryLayoutStaysInsideItsWorkArea(t *testing.T) {
	rng := newPropertyRand(1)
	ids := randomFramePresetIDs()
	for iteration := 0; iteration < propertyIterations; iteration++ {
		display := randomDisplay(rng, fmt.Sprintf("d%d", iteration))
		options := randomGaps(rng)
		window := randomWindow(rng, display)
		for _, id := range ids {
			layout := mustPresetForProperties(id)
			frame, _, err := Resolve(layout, display, window, options, nil)
			if err != nil {
				t.Fatalf("iteration %d preset %s: %v", iteration, id, err)
			}
			assertPropertyFrame(t, iteration, string(id), frame, display.WorkArea, window)
		}
		exact := randomExactLayout(rng)
		frame, _, err := Resolve(exact, display, window, options, nil)
		if err != nil {
			t.Fatalf("iteration %d exact: %v", iteration, err)
		}
		assertPropertyFrame(t, iteration, "exact", frame, display.WorkArea, window)
	}
}

// assertPropertyFrame checks the invariants for one resolved frame: finite, positive size, and inside the
// work area. An oversized non-resizable window keeps its size and so cannot fit; it is only held to
// positive size and a finite origin.
func assertPropertyFrame(t *testing.T, iteration int, name string, frame Rect, area Rect, window WindowFrame) {
	t.Helper()
	for _, value := range []float64{frame.X, frame.Y, frame.Width, frame.Height} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatalf("iteration %d %s: non-finite frame %+v", iteration, name, frame)
		}
	}
	if !frame.HasArea() {
		t.Fatalf("iteration %d %s: frame %+v has no area (work area %+v)", iteration, name, frame, area)
	}
	if !window.IsResizable && !fitsInside(window.Frame, area) {
		return
	}
	if frame.X < area.X || frame.Y < area.Y ||
		frame.X+frame.Width > area.X+area.Width || frame.Y+frame.Height > area.Y+area.Height {
		t.Fatalf("iteration %d %s: frame %+v outside work area %+v (window %+v)", iteration, name, frame, area, window)
	}
}

func TestPropertyPartitionsTileWithoutGapsOrOverlap(t *testing.T) {
	rng := newPropertyRand(2)
	groups := []tilingGroup{tilingGroups[0], tilingGroups[1], tilingGroups[2], tilingGroups[3], tilingGroups[4]}
	for iteration := 0; iteration < propertyIterations; iteration++ {
		display := randomDisplay(rng, "tile")
		area := display.WorkArea
		for _, group := range groups {
			frames := tiledFramesFor(t, group.layoutIDs, area)
			if got, want := totalArea(frames), area.Width*area.Height; got != want {
				t.Fatalf("iteration %d %s: area %v, want %v (work area %+v)", iteration, group.name, got, want, area)
			}
			if got := totalPairwiseOverlap(frames); got != 0 {
				t.Fatalf("iteration %d %s: overlap %v (work area %+v)", iteration, group.name, got, area)
			}
			for _, frame := range frames {
				if !isRectInside(frame, area) {
					t.Fatalf("iteration %d %s: tile %+v outside %+v", iteration, group.name, frame, area)
				}
			}
		}
	}
}

func TestPropertyHalvesKeepTheGapsExactly(t *testing.T) {
	rng := newPropertyRand(3)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		display := randomDisplay(rng, "gap")
		area := display.WorkArea
		options := randomGaps(rng)
		leftFrame := targetFrame(t, mustPreset(t, "left-half"), area, options, probeWindow)
		rightFrame := targetFrame(t, mustPreset(t, "right-half"), area, options, probeWindow)
		if leftFrame.X != area.X+options.OuterGap.Left {
			t.Fatalf("iteration %d: left edge %v, want %v", iteration, leftFrame.X, area.X+options.OuterGap.Left)
		}
		if rightFrame.X+rightFrame.Width != area.X+area.Width-options.OuterGap.Right {
			t.Fatalf("iteration %d: right edge %v, want %v", iteration, rightFrame.X+rightFrame.Width, area.X+area.Width-options.OuterGap.Right)
		}
		if leftFrame.X+leftFrame.Width+options.InnerGap != rightFrame.X {
			t.Fatalf("iteration %d: inner gap %v, want %v (outer %+v inner %v area %+v)", iteration,
				rightFrame.X-(leftFrame.X+leftFrame.Width), options.InnerGap, options.OuterGap, options.InnerGap, area)
		}
	}
}

func TestPropertyCycleReturnsToItsFirstStepAfterEachFullCycle(t *testing.T) {
	rng := newPropertyRand(4)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		display := randomDisplay(rng, "cycle")
		options := randomGaps(rng)
		options.Cycle = true
		options.CycleOrder = []CycleOrder{CycleHalfTwoThirdsThird, CycleHalfThirdTwoThirds}[rng.IntN(2)]
		group := []Edge{EdgeLeft, EdgeRight, EdgeTop, EdgeBottom}[rng.IntN(4)]
		layout := Layout{Kind: KindFraction, X: 0, Y: 0, Width: 0.5, Height: 1, CycleGroup: group}
		if group == EdgeRight {
			layout.X = 0.5
		}
		if group == EdgeTop || group == EdgeBottom {
			layout = Layout{Kind: KindFraction, X: 0, Y: 0, Width: 1, Height: 0.5, CycleGroup: group}
			if group == EdgeBottom {
				layout.Y = 0.5
			}
		}
		sim := newCycleSimulation(display.WorkArea)
		frames := make([]Rect, 0, 12)
		for press := 0; press < 12; press++ {
			frames = append(frames, sim.press(t, layout, options))
		}
		for press := 3; press < 12; press++ {
			if frames[press] != frames[press%3] {
				t.Fatalf("iteration %d press %d (%s, %v): %+v, want %+v", iteration, press+1, group, options.CycleOrder, frames[press], frames[press%3])
			}
		}
	}
}

// TestPropertyDisplayChoiceFollowsTheLargestOverlap checks PickDisplayForFrame against the overlap
// rule: the unique largest overlap wins; a tie or no overlap resolves to the main display.
func TestPropertyDisplayChoiceFollowsTheLargestOverlap(t *testing.T) {
	rng := newPropertyRand(5)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		displays := []Display{randomDisplay(rng, "a"), randomDisplay(rng, "b"), randomDisplay(rng, "c")}
		displays[1].Main = true
		frame := randomWindow(rng, displays[rng.IntN(3)]).Frame
		picked, err := PickDisplayForFrame(displays, frame)
		if err != nil {
			t.Fatal(err)
		}
		if want := expectedDisplayByOverlap(displays, frame); picked.ID != want {
			t.Fatalf("iteration %d: picked %q, want %q (frame %+v)", iteration, picked.ID, want, frame)
		}
	}
}

// expectedDisplayByOverlap is the specification of PickDisplayForFrame written independently of it.
func expectedDisplayByOverlap(displays []Display, frame Rect) string {
	best := 0.0
	var leaders []Display
	for _, display := range displays {
		area := intersectionArea(display.WorkArea, frame)
		switch {
		case area > best:
			best = area
			leaders = []Display{display}
		case area == best && area > 0:
			leaders = append(leaders, display)
		}
	}
	if best == 0 {
		return mainOrFirst(displays)
	}
	for _, leader := range leaders {
		if leader.Main {
			return leader.ID
		}
	}
	return leaders[0].ID
}

func mainOrFirst(displays []Display) string {
	for _, display := range displays {
		if display.Main {
			return display.ID
		}
	}
	return displays[0].ID
}

func TestPropertyExactLayoutsAreFinitePositiveAndInside(t *testing.T) {
	rng := newPropertyRand(6)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		display := randomDisplay(rng, "exact")
		layout := randomExactLayout(rng)
		if err := DescribeInvalidRequest(layout, DefaultOptions()); err != nil {
			t.Fatalf("iteration %d: generated an invalid exact layout: %v", iteration, err)
		}
		frame, _, err := Resolve(layout, display, resizableWindow(randomWindow(rng, display).Frame), DefaultOptions(), nil)
		if err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
		assertPropertyFrame(t, iteration, "exact", frame, display.WorkArea, resizableWindow(frame))
	}
}
