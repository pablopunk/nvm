package windowmgmt

import (
	"context"
	"fmt"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestListWindowsIssuesStableHandlesAndFiltersByAppAndTitle(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(0, 0, 100, 100))
	h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Title: "Notes", Frame: frameOf(0, 0, 100, 100)})
	h.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: otherBundle, AppName: "Other", Title: "Document", Frame: frameOf(0, 0, 100, 100)})
	ctx := context.Background()

	all, err := h.service.ListWindows(ctx, WindowQuery{})
	if err != nil || len(all) != 3 {
		t.Fatalf("ListWindows() = %d windows, %v; want 3", len(all), err)
	}
	byTitle, _ := h.service.ListWindows(ctx, WindowQuery{Title: "doc"})
	if len(byTitle) != 2 {
		t.Fatalf("title filter matched %d windows, want 2", len(byTitle))
	}
	byApp, _ := h.service.ListWindows(ctx, WindowQuery{AppID: "APP.OTHER"})
	if len(byApp) != 1 || byApp[0].AppID != otherBundle {
		t.Fatalf("app filter = %+v", byApp)
	}
	relisted, _ := h.service.ListWindows(ctx, WindowQuery{AppID: "APP.OTHER"})
	if relisted[0].Handle != byApp[0].Handle {
		t.Fatal("listing the same window twice issued a different handle")
	}
}

func TestListWindowsReportsAtMostMaxListedApps(t *testing.T) {
	h := newHarness(t)
	for index := range MaxListedApps + 5 {
		h.windows.AddWindow(fake.SimWindow{PID: 1000 + index, BundleID: fmt.Sprintf("app.%d", index), AppName: "App", Frame: frameOf(0, 0, 10, 10)})
	}

	listed, err := h.service.ListWindows(context.Background(), WindowQuery{})
	if err != nil || len(listed) != MaxListedApps {
		t.Fatalf("ListWindows() = %d windows, %v; want %d", len(listed), err, MaxListedApps)
	}
}

func TestApplyToListedWindowMovesItWithoutFocus(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(100, 100, 300, 200))
	listed := mustList(t, h, WindowQuery{AppID: targetBundle})

	result, err := h.service.ApplyToWindow(context.Background(), listed[0].Handle, presetLayout(t, "right-half"), geometry.UniformGaps(0))
	if err != nil {
		t.Fatalf("ApplyToWindow: %v", err)
	}
	if result.Frame != rectOf(500, 0, 500, 800) || h.focus.handedBack {
		t.Fatalf("result = %+v, handedBack = %v", result, h.focus.handedBack)
	}
}

func TestHandleOfAClosedWindowFailsTargetLostThenIsForgotten(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	listed := mustList(t, h, WindowQuery{AppID: targetBundle})
	h.windows.Remove(key)
	ctx := context.Background()

	_, err := h.service.ApplyToWindow(ctx, listed[0].Handle, presetLayout(t, "right-half"), geometry.UniformGaps(0))
	requireFailure(t, err, ReasonTargetLost)
	_, err = h.service.ApplyToWindow(ctx, listed[0].Handle, presetLayout(t, "right-half"), geometry.UniformGaps(0))
	requireFailure(t, err, ReasonUnknownWindow)
}

func TestHandlesExpireAfterTheirLifetime(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(100, 100, 300, 200))
	listed := mustList(t, h, WindowQuery{AppID: targetBundle})
	h.clock.Advance(handleTTL)

	err := h.service.FocusWindow(context.Background(), listed[0].Handle)
	requireFailure(t, err, ReasonUnknownWindow)
}

func TestFocusWindowRaisesTheListedWindow(t *testing.T) {
	h := newHarness(t)
	h.targetWindow(frameOf(0, 0, 400, 800))
	second := h.targetWindow(frameOf(500, 0, 400, 800))
	listed := mustList(t, h, WindowQuery{AppID: targetBundle})
	ctx := context.Background()

	if err := h.service.FocusWindow(ctx, listed[1].Handle); err != nil {
		t.Fatalf("FocusWindow: %v", err)
	}
	focused, err := h.windows.FocusedWindow(ctx, targetPID)
	if err != nil || focused.Key != second {
		t.Fatalf("focused window = %q (%v), want %q", focused.Key, err, second)
	}
}

func TestListDisplaysReportsWorkAreasAndNeedsAccess(t *testing.T) {
	left := testDisplay("left", 0, 0, 1000, 800, true)
	right := testDisplay("right", 1000, 0, 1000, 800, false)
	h := newHarness(t, left, right)
	ctx := context.Background()

	displays, err := h.service.ListDisplays(ctx)
	if err != nil || len(displays) != 2 || !displays[0].Main || displays[1].WorkArea != rectOf(1000, 0, 1000, 800) {
		t.Fatalf("ListDisplays = %+v, %v", displays, err)
	}
	h.windows.SetAccess(platform.WindowAccessDenied)
	_, err = h.service.ListDisplays(ctx)
	requireFailure(t, err, ReasonDenied)
}

func mustList(t *testing.T, h *harness, query WindowQuery) []WindowDescriptor {
	t.Helper()
	listed, err := h.service.ListWindows(context.Background(), query)
	if err != nil || len(listed) == 0 {
		t.Fatalf("ListWindows(%+v) = %d windows, %v", query, len(listed), err)
	}
	return listed
}
