package geometry

import (
	"errors"
	"testing"
)

// Ported from src/app/electron/os/window-layout-kinds.test.ts.

var (
	kindsWorkArea     = rectOf(0, 0, 1440, 900)
	kindsWindow       = rectOf(100, 120, 800, 500)
	centeredWindow    = rectOf(320, 200, 800, 500)
	negativeWorkArea  = rectOf(-1920, 0, 1920, 1080)
	kindsCenterWindow = centeredWindow
)

type engineCase struct {
	name     string
	layout   Layout
	expected Rect
	workArea *Rect
	gap      float64
	current  *Rect
}

func (c engineCase) resolve(t *testing.T) Rect {
	t.Helper()
	workArea := kindsWorkArea
	if c.workArea != nil {
		workArea = *c.workArea
	}
	current := kindsWindow
	if c.current != nil {
		current = *c.current
	}
	return targetFrame(t, c.layout, workArea, UniformGaps(c.gap), current)
}

func withWorkArea(rect Rect) *Rect { return &rect }

func withCurrent(rect Rect) *Rect { return &rect }

var maximizeWidthCases = []engineCase{
	{name: "maximize-width keeps vertical position and height", layout: Layout{Kind: KindMaximizeWidth}, expected: rectOf(0, 120, 1440, 500)},
	{name: "maximize-width insets horizontally by the gap", layout: Layout{Kind: KindMaximizeWidth}, gap: 10, expected: rectOf(10, 120, 1420, 500)},
	{name: "maximize-width pulls an offscreen window back vertically", layout: Layout{Kind: KindMaximizeWidth}, current: withCurrent(rectOf(5000, 5000, 400, 300)), expected: rectOf(0, 600, 1440, 300)},
	{name: "maximize-width clamps a window taller than the work area", layout: Layout{Kind: KindMaximizeWidth}, current: withCurrent(rectOf(0, -300, 400, 1200)), expected: rectOf(0, 0, 1440, 900)},
	{name: "maximize-width on a negative-origin display with gap 10", layout: Layout{Kind: KindMaximizeWidth}, workArea: withWorkArea(negativeWorkArea), gap: 10, expected: rectOf(-1910, 120, 1900, 500)},
}

var maximizeHeightCases = []engineCase{
	{name: "maximize-height keeps horizontal position and width", layout: Layout{Kind: KindMaximizeHeight}, expected: rectOf(100, 0, 800, 900)},
	{name: "maximize-height insets vertically by the gap", layout: Layout{Kind: KindMaximizeHeight}, gap: 10, expected: rectOf(100, 10, 800, 880)},
	{name: "maximize-height pulls an offscreen window back horizontally", layout: Layout{Kind: KindMaximizeHeight}, current: withCurrent(rectOf(5000, 5000, 400, 300)), expected: rectOf(1040, 0, 400, 900)},
}

var scaledCenterCases = []engineCase{
	{name: "reasonable-size is not capped on a 3000x2000 display", layout: mustPresetForKinds("reasonable-size"), workArea: withWorkArea(rectOf(0, 0, 3000, 2000)), expected: rectOf(450, 300, 2100, 1400)},
	{name: "reasonable-size scales a 1440x900 display to 70 percent", layout: mustPresetForKinds("reasonable-size"), expected: rectOf(216, 135, 1008, 630)},
	{name: "almost-maximize centers 90 percent of the work area", layout: mustPresetForKinds("almost-maximize"), expected: rectOf(72, 45, 1296, 810)},
	{name: "almost-maximize ignores the gap", layout: mustPresetForKinds("almost-maximize"), gap: 10, expected: rectOf(72, 45, 1296, 810)},
	{name: "almost-maximize on a negative-origin display", layout: mustPresetForKinds("almost-maximize"), workArea: withWorkArea(negativeWorkArea), expected: rectOf(-1824, 54, 1728, 972)},
	{name: "reasonable-size on a portrait display", layout: mustPresetForKinds("reasonable-size"), workArea: withWorkArea(rectOf(-1080, 0, 1080, 1920)), expected: rectOf(-918, 288, 756, 1344)},
	{name: "almost-maximize rounds an odd work area width", layout: mustPresetForKinds("almost-maximize"), workArea: withWorkArea(rectOf(0, 0, 1441, 900)), expected: rectOf(72, 45, 1297, 810)},
}

var edgeCases = []engineCase{
	{name: "move-left keeps size at gap 0", layout: mustPresetForKinds("move-left"), expected: rectOf(0, 120, 800, 500)},
	{name: "move-left insets by the gap", layout: mustPresetForKinds("move-left"), gap: 10, expected: rectOf(10, 120, 800, 500)},
	{name: "move-right at gap 0", layout: mustPresetForKinds("move-right"), expected: rectOf(640, 120, 800, 500)},
	{name: "move-right insets by the gap", layout: mustPresetForKinds("move-right"), gap: 10, expected: rectOf(630, 120, 800, 500)},
	{name: "move-up at gap 0", layout: mustPresetForKinds("move-up"), expected: rectOf(100, 0, 800, 500)},
	{name: "move-up insets by the gap", layout: mustPresetForKinds("move-up"), gap: 10, expected: rectOf(100, 10, 800, 500)},
	{name: "move-down at gap 0", layout: mustPresetForKinds("move-down"), expected: rectOf(100, 400, 800, 500)},
	{name: "move-down insets by the gap", layout: mustPresetForKinds("move-down"), gap: 10, expected: rectOf(100, 390, 800, 500)},
	{name: "move-left on a negative-origin display with gap 10", layout: mustPresetForKinds("move-left"), workArea: withWorkArea(negativeWorkArea), gap: 10, expected: rectOf(-1910, 120, 800, 500)},
	{name: "move-right on a negative-origin display with gap 10", layout: mustPresetForKinds("move-right"), workArea: withWorkArea(negativeWorkArea), gap: 10, expected: rectOf(-810, 120, 800, 500)},
	{name: "move-up on a display above the primary with gap 10", layout: mustPresetForKinds("move-up"), workArea: withWorkArea(rectOf(0, -1080, 1920, 1080)), gap: 10, expected: rectOf(100, -1070, 800, 500)},
	{name: "move-down on a display above the primary with gap 10", layout: mustPresetForKinds("move-down"), workArea: withWorkArea(rectOf(0, -1080, 1920, 1080)), gap: 10, expected: rectOf(100, -510, 800, 500)},
	{name: "move-right shrinks an oversized window to the work area", layout: mustPresetForKinds("move-right"), current: withCurrent(rectOf(0, 0, 2000, 1200)), gap: 10, expected: rectOf(0, 0, 1440, 900)},
	{name: "move-left pulls an offscreen window back vertically", layout: mustPresetForKinds("move-left"), current: withCurrent(rectOf(5000, 5000, 400, 300)), expected: rectOf(0, 600, 400, 300)},
	{name: "move-up pulls an offscreen window back horizontally", layout: mustPresetForKinds("move-up"), current: withCurrent(rectOf(5000, 5000, 400, 300)), expected: rectOf(1040, 0, 400, 300)},
}

var resizeStepCases = []engineCase{
	{name: "make-larger grows 5 percent of the work area around the center", layout: mustPresetForKinds("make-larger"), current: withCurrent(kindsCenterWindow), expected: rectOf(284, 178, 872, 545)},
	{name: "larger step 0.1 keeps the center", layout: Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: 0.1}, current: withCurrent(kindsCenterWindow), expected: rectOf(248, 155, 944, 590)},
	{name: "smaller step 0.1 keeps the center", layout: Layout{Kind: KindResizeStep, Direction: DirectionSmaller, Step: 0.1}, current: withCurrent(kindsCenterWindow), expected: rectOf(392, 245, 656, 410)},
	{name: "larger clamps to the work area", layout: Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: 0.1}, current: withCurrent(rectOf(0, 0, 1400, 880)), expected: rectOf(0, 0, 1440, 900)},
	{name: "larger shifts a grown window back inside the work area", layout: Layout{Kind: KindResizeStep, Direction: DirectionLarger, Step: 0.1}, current: withCurrent(rectOf(1300, 700, 100, 100)), expected: rectOf(1196, 655, 244, 190)},
	{name: "smaller never goes below 100 points", layout: Layout{Kind: KindResizeStep, Direction: DirectionSmaller, Step: 0.1}, current: withCurrent(rectOf(500, 300, 120, 120)), expected: rectOf(510, 310, 100, 100)},
	{name: "smaller never grows an already small window", layout: Layout{Kind: KindResizeStep, Direction: DirectionSmaller, Step: 0.1}, current: withCurrent(rectOf(500, 300, 80, 80)), expected: rectOf(500, 300, 80, 80)},
	{name: "larger on a negative-origin display", layout: mustPresetForKinds("make-larger"), workArea: withWorkArea(negativeWorkArea), current: withCurrent(rectOf(-1500, 300, 600, 400)), expected: rectOf(-1548, 273, 696, 454)},
}

// mustPresetForKinds returns a catalog layout for table construction; the catalog is static, so a
// missing ID is a programming error caught by the catalog tests.
func mustPresetForKinds(id PresetID) Layout {
	layout, ok := Preset(id)
	if !ok {
		panic("missing preset " + string(id))
	}
	return layout
}

func TestEngineCasesPort(t *testing.T) {
	var all []engineCase
	all = append(all, maximizeWidthCases...)
	all = append(all, maximizeHeightCases...)
	all = append(all, scaledCenterCases...)
	all = append(all, edgeCases...)
	all = append(all, resizeStepCases...)
	for _, c := range all {
		t.Run(c.name, func(t *testing.T) {
			assertFrame(t, c.resolve(t), c.expected)
		})
	}
}

func TestServiceHandledKindsAreRejectedByTheEngine(t *testing.T) {
	serviceHandled := []struct {
		name   string
		layout Layout
	}{
		{"restore", Layout{Kind: KindRestore}},
		{"display", Layout{Kind: KindDisplay, Direction: DirectionNext}},
		{"display", Layout{Kind: KindDisplay, Direction: DirectionPrevious}},
		{"toggle-full-screen", Layout{Kind: KindToggleFullScreen}},
	}
	for _, c := range serviceHandled {
		t.Run(c.name+" is rejected by the engine as service-handled", func(t *testing.T) {
			if !IsServiceHandled(c.layout.Kind) {
				t.Fatal("expected the kind to be service-handled")
			}
			_, err := computeTargetFrame(c.layout, kindsWorkArea, kindsWindow, UniformGaps(0))
			want := string(c.layout.Kind) + " is handled by the window management service"
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %q", err, want)
			}
		})
	}
}

func TestGeometryKindsAreNotServiceHandled(t *testing.T) {
	if IsServiceHandled(KindCenter) {
		t.Fatal("center must not be service-handled")
	}
	if IsServiceHandled(KindEdge) {
		t.Fatal("edge must not be service-handled")
	}
}

func TestFractionCycleGroupDoesNotChangeTheComputedFrame(t *testing.T) {
	layout := Layout{Kind: KindFraction, X: 0, Y: 0, Width: 0.5, Height: 1, CycleGroup: EdgeLeft}
	frame, _, err := Resolve(layout, displayOver("primary", kindsWorkArea), resizableWindow(kindsWindow), UniformGaps(0), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertFrame(t, frame, rectOf(0, 0, 720, 900))
}

func TestResolveRejectsDegenerateWorkArea(t *testing.T) {
	_, _, err := Resolve(mustPresetForKinds("maximize"), displayOver("zero", rectOf(0, 0, 0, 900)), resizableWindow(kindsWindow), UniformGaps(0), nil)
	if !errors.Is(err, ErrNoWorkArea) {
		t.Fatalf("error = %v, want ErrNoWorkArea", err)
	}
}
