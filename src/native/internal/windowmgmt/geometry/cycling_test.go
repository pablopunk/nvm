package geometry

import (
	"reflect"
	"testing"
)

// Ported from 1.x/electron/os/window-management-cycling.test.ts. The TypeScript cases drive the
// service through a simulated window. Here the same press sequence runs as pure geometry: resolve the
// cycle step, resolve the frame, remember the application. The service-level variant is WP4.1b.

// cycleWorkArea is the menu-bar-inset work area the simulated window lives on.
var cycleWorkArea = rectOf(0, 25, 1440, 875)

// cycleSimulation is one window and its history, as the engine would hold them.
type cycleSimulation struct {
	frame   Rect
	history *History
}

func newCycleSimulation(frame Rect) *cycleSimulation {
	return &cycleSimulation{frame: frame}
}

// press applies a layout the way a palette or shortcut command does and returns the new frame.
func (s *cycleSimulation) press(t *testing.T, layout Layout, options Options) Rect {
	t.Helper()
	window := resizableWindow(s.frame)
	var last *AppliedLayout
	if s.history != nil {
		last = s.history.LastApplied
	}
	chosen, cycle, err := ResolveCycleStep(layout, last, window, options)
	if err != nil {
		t.Fatalf("ResolveCycleStep: %v", err)
	}
	frame, _, err := Resolve(chosen, displayOver("main", cycleWorkArea), window, options, s.history)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	changed := !FramesMatch(frame, s.frame, ExactTolerancePoints)
	next := RememberApplied(s.history, window, frame, changed, cycle)
	s.history = &next
	s.frame = frame
	return frame
}

var (
	leftCycle = Layout{Kind: KindFraction, X: 0, Y: 0, Width: 0.5, Height: 1, CycleGroup: EdgeLeft}
	leftPlain = Layout{Kind: KindFraction, X: 0, Y: 0, Width: 0.5, Height: 1}
)

func cycleOptionsWithCycle(enabled bool) Options {
	options := UniformGaps(0)
	options.Cycle = enabled
	return options
}

func widthsOf(t *testing.T, sim *cycleSimulation, layout Layout, options Options, presses int) []int {
	t.Helper()
	widths := make([]int, 0, presses)
	for index := 0; index < presses; index++ {
		widths = append(widths, int(sim.press(t, layout, options).Width))
	}
	return widths
}

func TestCycleThreePressesPort(t *testing.T) {
	t.Run("three presses of the same half cycle through half, two-thirds and third", func(t *testing.T) {
		sim := newCycleSimulation(rectOf(100, 100, 600, 400))
		got := widthsOf(t, sim, leftCycle, cycleOptionsWithCycle(true), 4)
		if want := []int{720, 960, 480, 720}; !reflect.DeepEqual(got, want) {
			t.Fatalf("widths = %v, want %v", got, want)
		}
	})

	t.Run("cycleOrder half-third-two-thirds presses half, third and two-thirds", func(t *testing.T) {
		sim := newCycleSimulation(rectOf(100, 100, 600, 400))
		options := cycleOptionsWithCycle(true)
		options.CycleOrder = CycleHalfThirdTwoThirds
		got := widthsOf(t, sim, leftCycle, options, 4)
		if want := []int{720, 480, 960, 720}; !reflect.DeepEqual(got, want) {
			t.Fatalf("widths = %v, want %v", got, want)
		}
	})

	t.Run("a plain half command without a cycle group ends the cycle", func(t *testing.T) {
		sim := newCycleSimulation(rectOf(100, 100, 600, 400))
		options := cycleOptionsWithCycle(true)
		sim.press(t, leftCycle, options)
		sim.press(t, leftPlain, options)
		if got := int(sim.press(t, leftCycle, options).Width); got != 720 {
			t.Fatalf("width after the plain half = %v, want 720", got)
		}
	})

	t.Run("moving the window by hand restarts the cycle at the half", func(t *testing.T) {
		sim := newCycleSimulation(rectOf(100, 100, 600, 400))
		options := cycleOptionsWithCycle(true)
		sim.press(t, leftCycle, options)
		sim.frame = rectOf(300, 25, 720, 875)
		frame := sim.press(t, leftCycle, options)
		if frame.Width != 720 || frame.X != 0 {
			t.Fatalf("frame after manual move = %+v, want the left half", frame)
		}
	})

	t.Run("cycle:false keeps every press on the half", func(t *testing.T) {
		sim := newCycleSimulation(rectOf(100, 100, 600, 400))
		options := cycleOptionsWithCycle(false)
		if got := int(sim.press(t, leftCycle, options).Width); got != 720 {
			t.Fatalf("first press width = %v, want 720", got)
		}
		if got := int(sim.press(t, leftCycle, options).Width); got != 720 {
			t.Fatalf("second press width = %v, want 720", got)
		}
	})
}
