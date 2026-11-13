package windowmgmt

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestExtensionOptionsStartFromThePaletteDefaults(t *testing.T) {
	gap := 8
	options := optionsOf(api.WindowManagementOptions{InnerGap: &gap})
	defaults := geometry.DefaultOptions()
	if options.InnerGap != 8 || options.OuterGap != defaults.OuterGap || options.Cycle != defaults.Cycle {
		t.Fatalf("options = %+v, want the defaults with the inner gap set", options)
	}
}

func TestExtensionOptionsReachEveryField(t *testing.T) {
	cycle := false
	options := optionsOf(api.WindowManagementOptions{
		Cycle: &cycle, CycleOrder: "half-third-two-thirds", DisplayOrder: "top-to-bottom", DisplayMoveSizing: "keep-size",
	})
	if options.Cycle || options.CycleOrder != geometry.CycleHalfThirdTwoThirds || options.DisplayOrder != geometry.DisplayOrderTopToBottom || options.DisplayMoveSizing != geometry.MoveKeepSize {
		t.Fatalf("options = %+v, want each set field applied", options)
	}
}

func TestExtensionOutcomeNamesMatchTheAPI(t *testing.T) {
	cases := map[geometry.Outcome]string{geometry.Applied: "exact", geometry.Partial: "partial", geometry.Failed: "failed"}
	for outcome, want := range cases {
		if got := outcomeName(outcome); got != want {
			t.Errorf("outcomeName(%v) = %q, want %q", outcome, got, want)
		}
	}
}

func TestExtensionLayoutNeedsItsRectangle(t *testing.T) {
	_, err := layoutOf(api.WindowLayout{Kind: api.LayoutFraction})
	if err == nil {
		t.Fatal("a fraction layout without its rectangle was accepted")
	}
	if _, err := layoutOf(api.WindowLayout{Kind: api.LayoutExact}); err == nil {
		t.Fatal("an exact layout without its frame was accepted")
	}
}

// extensionFor is the extension view of h, acting on the app in front as a shortcut does.
func extensionFor(h *harness) Extension {
	return NewExtension(h.service, func(context.Context) Invocation { return *h.shortcutTarget() })
}

func extensionWithoutTarget(h *harness) Extension {
	return NewExtension(h.service, func(context.Context) Invocation { return Invocation{Source: SourceShortcut} })
}

func TestExtensionAppliesEveryLayoutKindToTheFrontmostWindow(t *testing.T) {
	cases := map[string]api.WindowLayout{
		"center":          {Kind: api.LayoutCenter},
		"maximize-width":  {Kind: api.LayoutMaximizeWidth},
		"maximize-height": {Kind: api.LayoutMaximizeHeight},
		"scaled-center":   {Kind: api.LayoutScaledCenter, WidthFraction: 0.5, HeightFraction: 0.5},
		"edge":            {Kind: api.LayoutEdge, Edge: "right"},
		"resize-step":     {Kind: api.LayoutResizeStep, Direction: "larger", Step: 0.05},
		"fraction":        {Kind: api.LayoutFraction, Fraction: &api.Rect{X: 0, Y: 0, Width: 0.5, Height: 1}},
		"exact":           {Kind: api.LayoutExact, Exact: &api.ExactFrame{Width: &api.Dimension{Value: 300, Unit: "points"}}},
	}
	for name, layout := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			key := h.targetWindow(frameOf(100, 100, 300, 200))
			h.windows.Focus(key)
			result, err := extensionFor(h).Apply(context.Background(), layout, api.WindowManagementOptions{})
			if err != nil || !result.OK {
				t.Fatalf("Apply(%s) = %+v, %v; want OK", name, result, err)
			}
			if result.Frame.Width <= 0 || result.Frame.Height <= 0 {
				t.Fatalf("Apply(%s) reported frame %+v", name, result.Frame)
			}
		})
	}
}

func TestExtensionApplyReportsARefusedRectangleAsPartial(t *testing.T) {
	h := newHarness(t)
	key := h.windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: targetBundle, AppName: "Target", Frame: frameOf(100, 100, 300, 200), MinWidth: 600})
	h.windows.Focus(key)
	result, err := extensionFor(h).Apply(context.Background(), api.WindowLayout{Kind: api.LayoutExact, Exact: &api.ExactFrame{Width: &api.Dimension{Value: 200, Unit: "points"}}}, api.WindowManagementOptions{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !result.OK || result.Outcome != "partial" || result.Reason == "" {
		t.Fatalf("result = %+v, want OK with outcome partial and a reason", result)
	}
}

func TestExtensionRejectsInvalidLayoutsAsResults(t *testing.T) {
	invalid := map[string]api.WindowLayout{
		"fraction without rect": {Kind: api.LayoutFraction},
		"exact without frame":   {Kind: api.LayoutExact},
		"step out of range":     {Kind: api.LayoutResizeStep, Direction: "larger", Step: 0.5},
		"unknown kind":          {Kind: "spiral"},
	}
	for name, layout := range invalid {
		h := newHarness(t)
		h.windows.Focus(h.targetWindow(frameOf(100, 100, 300, 200)))
		result, err := extensionFor(h).Apply(context.Background(), layout, api.WindowManagementOptions{})
		if err != nil || result.OK || result.Reason != string(ReasonInvalidSpec) {
			t.Errorf("%s: result = %+v, %v; want invalid-spec as a result", name, result, err)
		}
	}
}

func TestExtensionApplyWithoutATargetIsAResult(t *testing.T) {
	h := newHarness(t)
	h.windows.Focus(h.targetWindow(frameOf(100, 100, 300, 200)))
	result, err := extensionWithoutTarget(h).Apply(context.Background(), api.WindowLayout{Kind: api.LayoutCenter}, api.WindowManagementOptions{})
	if err != nil || result.OK || result.Reason != string(ReasonNoTarget) {
		t.Fatalf("result = %+v, %v; want no-target as a result", result, err)
	}
}

func TestExtensionAppliesToAListedWindow(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(100, 100, 300, 200))
	h.windows.Focus(key)
	ctx := context.Background()
	listed, err := extensionFor(h).Windows(ctx, api.WindowQuery{AppName: "Target"})
	if err != nil || len(listed) != 1 {
		t.Fatalf("Windows = %+v, %v; want the target window", listed, err)
	}
	result, err := extensionFor(h).ApplyTo(ctx, listed[0].Handle, api.WindowLayout{Kind: api.LayoutMaximizeWidth}, api.WindowManagementOptions{})
	if err != nil || !result.OK || result.Frame.Width <= 300 {
		t.Fatalf("ApplyTo = %+v, %v; want the window widened", result, err)
	}
}

func TestExtensionFocusesAListedWindowAndRefusesAStaleOne(t *testing.T) {
	h := newHarness(t)
	h.windows.Focus(h.targetWindow(frameOf(0, 0, 400, 800)))
	other := h.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: otherBundle, AppName: "Other", Frame: frameOf(500, 0, 400, 800)})
	ctx := context.Background()
	listed, err := extensionFor(h).Windows(ctx, api.WindowQuery{AppName: "Other"})
	if err != nil || len(listed) != 1 {
		t.Fatalf("Windows = %+v, %v", listed, err)
	}
	if result, err := extensionFor(h).Focus(ctx, listed[0].Handle); err != nil || !result.OK {
		t.Fatalf("Focus = %+v, %v; want OK", result, err)
	}
	if front, _, _ := h.windows.Frontmost(ctx); front.PID != otherPID {
		t.Fatalf("frontmost PID %d after Focus, want the other app (%s)", front.PID, other)
	}
	stale, err := extensionFor(h).Focus(ctx, "no-such-handle")
	if err != nil || stale.OK || stale.Reason != string(ReasonUnknownWindow) {
		t.Fatalf("stale Focus = %+v, %v; want unknown-window as a result", stale, err)
	}
}

func TestExtensionFocusNeighborMovesRightAndRefusesAnUnknownDirection(t *testing.T) {
	h := newHarness(t)
	left := h.targetWindow(frameOf(0, 0, 400, 800))
	h.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: otherBundle, AppName: "Other", Frame: frameOf(500, 0, 400, 800)})
	h.windows.Focus(left)
	ctx := context.Background()

	if result, err := extensionFor(h).FocusNeighbor(ctx, "right", false); err != nil || !result.OK {
		t.Fatalf("FocusNeighbor(right) = %+v, %v; want OK", result, err)
	}
	if result, err := extensionFor(h).FocusNeighbor(ctx, "sideways", false); err != nil || result.OK || result.Reason != string(ReasonInvalidSpec) {
		t.Fatalf("FocusNeighbor(sideways) = %+v, %v; want invalid-spec", result, err)
	}
}

func TestExtensionFocusNeighborReportsNoNeighborAsAResult(t *testing.T) {
	h := newHarness(t)
	h.windows.Focus(h.targetWindow(frameOf(0, 0, 400, 800)))
	result, err := extensionFor(h).FocusNeighbor(context.Background(), "left", false)
	if err != nil || result.OK || result.Reason != string(ReasonNoNeighbor) {
		t.Fatalf("FocusNeighbor(left) = %+v, %v; want no-neighbor as a result", result, err)
	}
}

func TestExtensionListsDisplaysAndWindows(t *testing.T) {
	h := newHarness(t)
	h.windows.Focus(h.targetWindow(frameOf(100, 100, 300, 200)))
	ctx := context.Background()
	displays, err := extensionFor(h).Displays(ctx)
	if err != nil || len(displays) != 1 || !displays[0].Primary || displays[0].WorkArea.Width != 1000 {
		t.Fatalf("Displays = %+v, %v", displays, err)
	}
	windows, err := extensionFor(h).Windows(ctx, api.WindowQuery{})
	if err != nil || len(windows) != 1 || windows[0].AppName != "Target" {
		t.Fatalf("Windows = %+v, %v", windows, err)
	}
}
