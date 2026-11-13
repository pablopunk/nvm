package geometry

import (
	"errors"
	"reflect"
	"testing"
)

// Ported from 1.x/electron/os/window-display-geometry.test.ts (31 cases) and the pure parts of
// 1.x/electron/os/window-management-display.test.ts (the service-harness cases are deferred).

var (
	geoPrimary = Display{ID: "primary", Main: true, Scale: 1, Frame: rectOf(0, 0, 1440, 900), WorkArea: rectOf(0, 0, 1440, 900)}
	geoRight   = displayOver("right", rectOf(1440, 0, 1920, 1080))
	geoLeft    = displayOver("left", rectOf(-1920, 0, 1920, 1080))
	geoAbove   = displayOver("above", rectOf(0, -1080, 1920, 1080))
	geoFullHD  = displayOver("full-hd", rectOf(0, 0, 1920, 1080))
	geoSmallL  = displayOver("small-left", rectOf(-1440, 0, 1440, 900))
)

type adjacentCase struct {
	name      string
	displays  []Display
	current   string
	direction Direction
	expected  string
	found     bool
}

func displayByID(displays []Display, id string) Display {
	for _, display := range displays {
		if display.ID == id {
			return display
		}
	}
	return displayOver(id, geoPrimary.Frame)
}

func TestAdjacentDisplayPort(t *testing.T) {
	cases := []adjacentCase{
		{name: "single display has no next neighbour", displays: []Display{geoPrimary}, current: "primary", direction: DirectionNext},
		{name: "single display has no previous neighbour", displays: []Display{geoPrimary}, current: "primary", direction: DirectionPrevious},
		{name: "two displays: next from primary", displays: []Display{geoPrimary, geoRight}, current: "primary", direction: DirectionNext, expected: "right", found: true},
		{name: "two displays: next wraps from the rightmost", displays: []Display{geoPrimary, geoRight}, current: "right", direction: DirectionNext, expected: "primary", found: true},
		{name: "two displays: previous from primary", displays: []Display{geoPrimary, geoRight}, current: "primary", direction: DirectionPrevious, expected: "right", found: true},
		{name: "three displays in shuffled input: next wraps from the rightmost to the leftmost", displays: []Display{geoRight, geoPrimary, geoLeft}, current: "right", direction: DirectionNext, expected: "left", found: true},
		{name: "three displays: previous wraps from the leftmost to the rightmost", displays: []Display{geoRight, geoPrimary, geoLeft}, current: "left", direction: DirectionPrevious, expected: "right", found: true},
		{name: "three displays: next from the leftmost reaches the primary", displays: []Display{geoPrimary, geoLeft, geoRight}, current: "left", direction: DirectionNext, expected: "primary", found: true},
		{name: "ties on x are ordered by y: next from the display above wraps to primary", displays: []Display{geoPrimary, geoRight, geoAbove}, current: "above", direction: DirectionNext, expected: "primary", found: true},
		{name: "ties on x are ordered by y: previous from the display above wraps to the rightmost", displays: []Display{geoRight, geoPrimary, geoAbove}, current: "above", direction: DirectionPrevious, expected: "right", found: true},
		{name: "ties on x are ordered by y: next from primary reaches the rightmost", displays: []Display{geoAbove, geoRight, geoPrimary}, current: "primary", direction: DirectionNext, expected: "right", found: true},
		{name: "unknown current display has no neighbour", displays: []Display{geoPrimary, geoRight}, current: "missing", direction: DirectionNext},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := AdjacentDisplay(c.displays, displayByID(c.displays, c.current), c.direction, DisplayOrderLeftToRight)
			if ok != c.found || (ok && got.ID != c.expected) {
				t.Fatalf("got (%q, %v), want (%q, %v)", got.ID, ok, c.expected, c.found)
			}
		})
	}
}

var stackedDisplays = []Display{geoPrimary, geoAbove, geoLeft, geoRight}

func TestAdjacentDisplayOrderingPort(t *testing.T) {
	t.Run("previous display from primary is the display above it when ordered left-to-right", func(t *testing.T) {
		got, ok := AdjacentDisplay(stackedDisplays, geoPrimary, DirectionPrevious, DisplayOrderLeftToRight)
		if !ok || got.ID != "above" {
			t.Fatalf("got (%q, %v), want above", got.ID, ok)
		}
	})
	t.Run("previous display from primary is the display to its left when ordered top-to-bottom", func(t *testing.T) {
		got, ok := AdjacentDisplay(stackedDisplays, geoPrimary, DirectionPrevious, DisplayOrderTopToBottom)
		if !ok || got.ID != "left" {
			t.Fatalf("got (%q, %v), want left", got.ID, ok)
		}
	})
	t.Run("next display from the rightmost display wraps to the top when ordered top-to-bottom", func(t *testing.T) {
		got, ok := AdjacentDisplay(stackedDisplays, geoRight, DirectionNext, DisplayOrderTopToBottom)
		if !ok || got.ID != "above" {
			t.Fatalf("got (%q, %v), want above", got.ID, ok)
		}
	})
	t.Run("adjacent display does not reorder the caller array", func(t *testing.T) {
		displays := []Display{geoRight, geoPrimary, geoLeft}
		_, _ = AdjacentDisplay(displays, geoPrimary, DirectionNext, DisplayOrderLeftToRight)
		ids := []string{displays[0].ID, displays[1].ID, displays[2].ID}
		if !reflect.DeepEqual(ids, []string{"right", "primary", "left"}) {
			t.Fatalf("caller order changed: %v", ids)
		}
	})
}

type moveCase struct {
	name     string
	frame    Rect
	from     Display
	to       Display
	sizing   DisplayMoveSizing
	expected Rect
}

func runMoveCases(t *testing.T, cases []moveCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sizing := c.sizing
			if sizing == "" {
				sizing = MoveProportional
			}
			got, err := MoveFrameToDisplay(c.frame, c.from, c.to, sizing)
			if err != nil {
				t.Fatal(err)
			}
			assertFrame(t, got, c.expected)
		})
	}
}

func TestMoveFrameToDisplayPort(t *testing.T) {
	runMoveCases(t, []moveCase{
		{name: "left half moves to a larger display on the left keeping half size", frame: rectOf(0, 0, 720, 900), from: geoPrimary, to: geoLeft, expected: rectOf(-1920, 0, 960, 1080)},
		{name: "window right of center scales proportionally to a larger display on the right", frame: rectOf(1000, 100, 400, 300), from: geoPrimary, to: geoRight, expected: rectOf(2773, 120, 533, 360)},
		{name: "window moves to a smaller display and clamps fully inside it", frame: rectOf(1700, 700, 400, 300), from: geoFullHD, to: geoSmallL, expected: rectOf(-300, 583, 300, 250)},
		{name: "window on a negative-origin display scales back to the primary", frame: rectOf(-1500, 100, 600, 400), from: geoLeft, to: geoPrimary, expected: rectOf(315, 83, 450, 333)},
		{name: "window moves to a display above the primary with a larger resolution", frame: rectOf(100, 120, 800, 500), from: geoPrimary, to: geoAbove, expected: rectOf(133, -936, 1067, 600)},
		{name: "window larger than the target display is capped to its work area", frame: rectOf(0, 0, 2000, 1200), from: geoFullHD, to: displayOver("small", rectOf(0, 0, 1000, 800)), expected: rectOf(0, 0, 1000, 800)},
		{name: "moving to the same display keeps the frame", frame: rectOf(100, 120, 800, 500), from: geoPrimary, to: geoPrimary, expected: rectOf(100, 120, 800, 500)},
	})
}

func TestMoveFrameToDisplayKeepSizePort(t *testing.T) {
	runMoveCases(t, []moveCase{
		{name: "keep-size keeps the window size and its relative center on a larger display", frame: rectOf(0, 0, 720, 900), from: geoPrimary, to: geoRight, sizing: MoveKeepSize, expected: rectOf(1560, 90, 720, 900)},
		{name: "keep-size caps the window to a smaller display and clamps it inside", frame: rectOf(0, 0, 1500, 1000), from: geoFullHD, to: geoPrimary, sizing: MoveKeepSize, expected: rectOf(0, 0, 1440, 900)},
	})
}

func TestCycleSequencePort(t *testing.T) {
	const third = 1.0 / 3
	const twoThirds = 2.0 / 3
	const half = 1.0 / 2
	fraction := func(x, y, width, height float64) Layout {
		return Layout{Kind: KindFraction, X: x, Y: y, Width: width, Height: height}
	}
	cases := []struct {
		group    Edge
		expected []Layout
	}{
		{EdgeLeft, []Layout{fraction(0, 0, half, 1), fraction(0, 0, twoThirds, 1), fraction(0, 0, third, 1)}},
		{EdgeRight, []Layout{fraction(half, 0, half, 1), fraction(third, 0, twoThirds, 1), fraction(twoThirds, 0, third, 1)}},
		{EdgeTop, []Layout{fraction(0, 0, 1, half), fraction(0, 0, 1, twoThirds), fraction(0, 0, 1, third)}},
		{EdgeBottom, []Layout{fraction(0, half, 1, half), fraction(0, third, 1, twoThirds), fraction(0, twoThirds, 1, third)}},
	}
	for _, c := range cases {
		t.Run(string(c.group)+" cycle runs half, two-thirds, third without cycleGroup", func(t *testing.T) {
			got, err := CycleSequence(c.group, CycleHalfTwoThirdsThird)
			if err != nil || !reflect.DeepEqual(got, c.expected) {
				t.Fatalf("got %+v (%v), want %+v", got, err, c.expected)
			}
		})
	}
	t.Run("left cycle runs half, third, two-thirds when the third step comes first", func(t *testing.T) {
		got, err := CycleSequence(EdgeLeft, CycleHalfThirdTwoThirds)
		want := []Layout{fraction(0, 0, half, 1), fraction(0, 0, third, 1), fraction(0, 0, twoThirds, 1)}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v (%v), want %+v", got, err, want)
		}
	})
	t.Run("bottom cycle runs half, third, two-thirds when the third step comes first", func(t *testing.T) {
		got, err := CycleSequence(EdgeBottom, CycleHalfThirdTwoThirds)
		want := []Layout{fraction(0, half, 1, half), fraction(0, twoThirds, 1, third), fraction(0, third, 1, twoThirds)}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v (%v), want %+v", got, err, want)
		}
	})
	t.Run("an unknown cycle group has no sequence", func(t *testing.T) {
		if _, err := CycleSequence("middle", CycleHalfTwoThirdsThird); !errors.Is(err, ErrUnknownCycle) {
			t.Fatalf("error = %v, want ErrUnknownCycle", err)
		}
	})
}

// The next and previous display harness cases, driven through the pure display operation.
var (
	harnessPrimary   = Display{ID: "primary", Main: true, Scale: 1, Frame: rectOf(0, 0, 1440, 900), WorkArea: rectOf(0, 25, 1440, 875)}
	harnessSecondary = Display{ID: "secondary", Scale: 1, Frame: rectOf(1440, 0, 1440, 900), WorkArea: rectOf(1440, 25, 1440, 875)}
	harnessOriginal  = rectOf(100, 100, 600, 400)
	onSecondaryFrame = rectOf(1540, 100, 600, 400)
)

func movedWindow(frame Rect) WindowFrame {
	return WindowFrame{Frame: frame, IsResizable: true}
}

func TestDisplayMoveHarnessCasesPort(t *testing.T) {
	displays := []Display{harnessPrimary, harnessSecondary}
	options := UniformGaps(0)

	t.Run("moves the window to the next display, wrapping back to the first", func(t *testing.T) {
		moved, to, err := MoveWindowToAdjacentDisplay(displays, movedWindow(harnessOriginal), DirectionNext, options)
		if err != nil || to.ID != harnessSecondary.ID {
			t.Fatalf("to = %v, err = %v", to.ID, err)
		}
		assertFrame(t, moved, onSecondaryFrame)
		back, _, err := MoveWindowToAdjacentDisplay(displays, movedWindow(moved), DirectionNext, options)
		if err != nil {
			t.Fatal(err)
		}
		assertFrame(t, back, harnessOriginal)
	})

	t.Run("moves the window to the previous display, wrapping to the last", func(t *testing.T) {
		moved, _, err := MoveWindowToAdjacentDisplay(displays, movedWindow(harnessOriginal), DirectionPrevious, options)
		if err != nil {
			t.Fatal(err)
		}
		assertFrame(t, moved, onSecondaryFrame)
	})

	t.Run("records the pre-move frame so restore brings the window back", func(t *testing.T) {
		moved, _, err := MoveWindowToAdjacentDisplay(displays, movedWindow(harnessOriginal), DirectionNext, options)
		if err != nil {
			t.Fatal(err)
		}
		history := RememberApplied(nil, movedWindow(harnessOriginal), moved, true, CycleRecord{})
		restore, _, err := Resolve(Layout{Kind: KindRestore}, harnessPrimary, movedWindow(moved), options, &history)
		if err != nil {
			t.Fatal(err)
		}
		assertFrame(t, restore, harnessOriginal)
	})

	t.Run("reports a single display without touching the window", func(t *testing.T) {
		_, _, err := MoveWindowToAdjacentDisplay([]Display{harnessPrimary}, movedWindow(harnessOriginal), DirectionNext, options)
		if !errors.Is(err, ErrSingleDisplay) {
			t.Fatalf("error = %v, want ErrSingleDisplay", err)
		}
		if err.Error() != "Only one display is connected." {
			t.Fatalf("message = %q", err.Error())
		}
	})
}
