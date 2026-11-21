package windowmgmt

import (
	"context"
	"math"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestRepeatedHalfCommandCyclesHalfTwoThirdsThird(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)
	ctx := context.Background()
	options := geometry.UniformGaps(0)

	var widths []float64
	for range 3 {
		result, err := h.service.Apply(ctx, presetLayout(t, "left-half"), options, *h.shortcutTarget())
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if result.Frame.X != 0 {
			t.Fatalf("cycle moved the window off the left edge: %v", result.Frame)
		}
		widths = append(widths, result.Frame.Width)
	}
	wantWidths := []float64{500, 1000 * 2 / 3.0, 1000 / 3.0}
	for index, want := range wantWidths {
		if math.Abs(widths[index]-want) > 1 {
			t.Fatalf("cycle widths = %v, want about %v", widths, wantWidths)
		}
	}
}

func TestUserMovingTheWindowRestartsTheCycle(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)
	ctx := context.Background()
	options := geometry.UniformGaps(0)

	if _, err := h.service.Apply(ctx, presetLayout(t, "left-half"), options, *h.shortcutTarget()); err != nil {
		t.Fatalf("first half: %v", err)
	}
	if _, err := h.windows.SetFrame(ctx, platform.WindowWrite{PID: targetPID, Key: key, Frame: frameOf(300, 300, 400, 300)}); err != nil {
		t.Fatalf("simulated user move: %v", err)
	}
	result, err := h.service.Apply(ctx, presetLayout(t, "left-half"), options, *h.shortcutTarget())
	if err != nil {
		t.Fatalf("second half: %v", err)
	}
	if result.Frame.Width != 500 {
		t.Fatalf("after a user move the command continued the cycle: width %v", result.Frame.Width)
	}
}

func TestRestoreReturnsTheWindowToItsFrameBeforeTheFirstLayout(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)
	ctx := context.Background()
	options := geometry.UniformGaps(0)

	if _, err := h.service.Apply(ctx, presetLayout(t, "left-half"), options, *h.shortcutTarget()); err != nil {
		t.Fatalf("layout: %v", err)
	}
	restored, err := h.service.Apply(ctx, geometry.Layout{Kind: geometry.KindRestore}, options, *h.shortcutTarget())
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Frame != rectOf(100, 100, 300, 200) {
		t.Fatalf("restored frame = %v", restored.Frame)
	}
	_, err = h.service.Apply(ctx, geometry.Layout{Kind: geometry.KindRestore}, options, *h.shortcutTarget())
	requireFailure(t, err, ReasonNoHistory)
}

func TestRestoreWithoutAnEarlierLayoutFails(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)

	_, err := h.service.Apply(context.Background(), geometry.Layout{Kind: geometry.KindRestore}, geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonNoHistory)
}

func TestDisplayMoveSendsTheWindowToTheNextDisplay(t *testing.T) {
	left := testDisplay("left", 0, 0, 1000, 800, true)
	right := testDisplay("right", 1000, 0, 1000, 800, false)
	h := newHarness(t, left, right)
	key := h.targetWindow(frameOf(100, 100, 400, 400))
	h.windows.Focus(key)

	next := geometry.Layout{Kind: geometry.KindDisplay, Direction: geometry.DirectionNext}
	result, err := h.service.Apply(context.Background(), next, geometry.UniformGaps(0), *h.shortcutTarget())
	if err != nil {
		t.Fatalf("display move: %v", err)
	}
	if result.Frame.X < 1000 || result.Frame.Width != 400 {
		t.Fatalf("frame = %v, want the window on the right display at its size", result.Frame)
	}
	if got := h.displayOf(key); got != "right" {
		t.Fatalf("window display = %q", got)
	}
}

func TestDisplayMoveWithOneDisplayFailsSingleDisplay(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 400, 400))
	h.windows.Focus(key)

	next := geometry.Layout{Kind: geometry.KindDisplay, Direction: geometry.DirectionNext}
	_, err := h.service.Apply(context.Background(), next, geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonSingleDisplay)
}

func TestFullScreenToggleEntersAndLeavesFullScreenAndReadsItBack(t *testing.T) {
	h := newHarness(t, testDisplay("main", 0, 0, 1000, 800, true))
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)
	toggle := geometry.Layout{Kind: geometry.KindToggleFullScreen}
	ctx := context.Background()

	entered, err := h.service.Apply(ctx, toggle, geometry.UniformGaps(0), *h.shortcutTarget())
	if err != nil {
		t.Fatalf("enter full screen: %v", err)
	}
	if !h.window(key).FullScreen || entered.Frame != rectOf(0, 0, 1000, 800) {
		t.Fatalf("after entering: window %+v, result %+v", h.window(key), entered)
	}
	if _, err := h.service.Apply(ctx, toggle, geometry.UniformGaps(0), *h.shortcutTarget()); err != nil {
		t.Fatalf("leave full screen: %v", err)
	}
	if h.window(key).FullScreen {
		t.Fatal("window is still full screen")
	}
}
