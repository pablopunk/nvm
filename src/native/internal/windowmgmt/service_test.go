package windowmgmt

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestPaletteCommandRestoresFocusThenMovesTheFocusedWindow(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.openedFromTarget(key)

	result, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.paletteTarget())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := rectOf(0, 0, 500, 800)
	if result.Outcome != geometry.Applied || result.Frame != want || !result.Changed {
		t.Fatalf("result = %+v, want applied %v", result, want)
	}
	if !h.focus.handedBack {
		t.Fatal("palette command did not hand focus back to the target app")
	}
	if got := h.window(key).Frame; got != frameOf(0, 0, 500, 800) {
		t.Fatalf("window frame = %+v", got)
	}
}

func TestPaletteCommandStopsWhenTheAppDoesNotTakeFocusBack(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	other := h.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: otherBundle, AppName: "Other", Frame: frameOf(0, 0, 10, 10)})
	h.windows.Focus(other)
	h.openedFromTarget("")

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.paletteTarget())
	failed := requireFailure(t, err, ReasonFocusNotRestored)
	if failed.Message != messageFocusNotRestored {
		t.Fatalf("message = %q", failed.Message)
	}
	if got := h.window(key).Frame; got != frameOf(100, 100, 300, 200) {
		t.Fatalf("window moved without focus: %+v", got)
	}
}

func TestPaletteCommandWithoutAPaletteTargetReportsTheChangedApp(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(100, 100, 300, 200))
	h.openedFromTarget("")
	h.focus.paletteApp = &Target{PID: otherPID, BundleID: otherBundle}

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.paletteTarget())
	failed := requireFailure(t, err, ReasonFocusNotRestored)
	if failed.Message != messageFocusChangedPalette {
		t.Fatalf("message = %q, want the changed-app copy", failed.Message)
	}
}

func TestShortcutCommandMovesWindowThatIsAlreadyInFront(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)

	result, err := h.service.Apply(context.Background(), presetLayout(t, "right-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Frame != rectOf(500, 0, 500, 800) {
		t.Fatalf("frame = %v", result.Frame)
	}
	if h.focus.handedBack {
		t.Fatal("shortcut command took focus back; shortcuts must never take focus")
	}
}

func TestShortcutCommandRefusesWhenTheTargetIsNotInFront(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	other := h.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: otherBundle, AppName: "Other", Frame: frameOf(0, 0, 10, 10)})
	h.windows.Focus(other)

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	failed := requireFailure(t, err, ReasonFocusNotRestored)
	if failed.Message != messageFocusChangedKey {
		t.Fatalf("message = %q", failed.Message)
	}
	if got := h.window(key).Frame; got != frameOf(100, 100, 300, 200) {
		t.Fatalf("window moved while another app was in front: %+v", got)
	}
}

func TestCommandsWithoutATargetOrOnNevermindItselfFailBeforeAnyWindowCall(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(100, 100, 300, 200))

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), Invocation{Source: SourcePalette})
	failed := requireFailure(t, err, ReasonNoTarget)
	if failed.Message != messageNoTargetPalette {
		t.Fatalf("message = %q", failed.Message)
	}

	self := Invocation{Source: SourceShortcut, Target: &Target{PID: selfPIDForTest, BundleID: "app.nevermind"}}
	_, err = h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), self)
	requireFailure(t, err, ReasonNoTarget)
	if calls := h.windows.Calls(); len(calls) != 0 {
		t.Fatalf("window system was called for an invalid command: %+v", calls)
	}
}

func TestServiceWithoutAWindowManagerIsUnsupported(t *testing.T) {
	service := NewService(Deps{Clock: fake.NewClock()})
	if service.Available() {
		t.Fatal("service without a window manager reports available")
	}
	_, err := service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), Invocation{Source: SourceShortcut, Target: &Target{PID: targetPID}})
	requireFailure(t, err, ReasonUnsupported)
}

func TestDeniedAccessChangesNoWindow(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.SetAccess(platform.WindowAccessDenied)
	h.windows.Focus(key)

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonDenied)
	if got := h.window(key).Frame; got != frameOf(100, 100, 300, 200) {
		t.Fatalf("window moved without access: %+v", got)
	}
}

func TestWindowThatRefusesTheFrameIsReportedUnapplied(t *testing.T) {
	h := newHarness(t)
	key := h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Frame: frameOf(100, 100, 300, 200), Refuses: true})
	h.windows.Focus(key)

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonUnapplied)
}

func TestMinimumSizePushingPastTheDisplayIsPartialAndPulledBackInside(t *testing.T) {
	h := newHarness(t)
	key := h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Frame: frameOf(100, 100, 300, 200), MinWidth: 600})
	h.windows.Focus(key)

	result, err := h.service.Apply(context.Background(), presetLayout(t, "right-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Outcome != geometry.Partial || result.Message == "" {
		t.Fatalf("result = %+v, want a partial result with a message", result)
	}
	if result.Frame != rectOf(400, 0, 600, 800) {
		t.Fatalf("frame = %v, want the size the app accepted pulled back inside the display", result.Frame)
	}
	last := h.windows.Calls()
	if !lastSetFrameWasPositionOnly(last) {
		t.Fatalf("no position-only correction was issued: %+v", last)
	}
}

func TestFixedWindowKeepsItsSizeAndAlignsToTheZoneEdge(t *testing.T) {
	h := newHarness(t)
	key := h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Frame: frameOf(100, 100, 300, 200), Fixed: true})
	h.windows.Focus(key)

	result, err := h.service.Apply(context.Background(), presetLayout(t, "right-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Frame != rectOf(700, 300, 300, 200) {
		t.Fatalf("frame = %v, want the fixed size anchored to the right edge and centred vertically", result.Frame)
	}
}

func TestFullScreenWindowRejectsLayoutsButAcceptsTheToggle(t *testing.T) {
	h := newHarness(t)
	key := h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Frame: frameOf(0, 0, 1000, 800), FullScreen: true})
	h.windows.Focus(key)

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonFullScreen)

	toggle := geometry.Layout{Kind: geometry.KindToggleFullScreen}
	if _, err := h.service.Apply(context.Background(), toggle, geometry.UniformGaps(0), *h.shortcutTarget()); err != nil {
		t.Fatalf("toggle out of full screen: %v", err)
	}
	if h.window(key).FullScreen {
		t.Fatal("window is still full screen after the toggle")
	}
}

func TestMinimizedWindowIsRejected(t *testing.T) {
	h := newHarness(t)
	key := h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Frame: frameOf(100, 100, 300, 200), Minimized: true})
	h.windows.Focus(key)

	_, err := h.service.Apply(context.Background(), presetLayout(t, "left-half"), geometry.UniformGaps(0), *h.shortcutTarget())
	requireFailure(t, err, ReasonMinimized)
}

func TestInvalidLayoutIsRejectedBeforeTheWindowSystemIsCalled(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(100, 100, 300, 200))
	invalid := geometry.Layout{Kind: geometry.KindResizeStep, Direction: geometry.DirectionLarger, Step: 5}

	_, err := h.service.Apply(context.Background(), invalid, geometry.UniformGaps(0), *h.paletteTarget())
	requireFailure(t, err, ReasonInvalidSpec)
	if calls := h.windows.Calls(); len(calls) != 0 {
		t.Fatalf("window system was called for an invalid layout: %+v", calls)
	}
}

func lastSetFrameWasPositionOnly(calls []fake.WindowsCall) bool {
	for index := len(calls) - 1; index >= 0; index-- {
		if calls[index].Op == "SetFrame" {
			return calls[index].PositionOnly
		}
	}
	return false
}
