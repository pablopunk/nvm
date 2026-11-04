package windowmgmt

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

func TestFocusNeighborRaisesTheNearestWindowInTheDirection(t *testing.T) {
	h := newHarness(t)
	left := h.targetWindow(frameOf(0, 0, 400, 800))
	right := h.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: otherBundle, AppName: "Other", Frame: frameOf(500, 0, 400, 800)})
	h.windows.Focus(left)
	ctx := context.Background()

	if err := h.service.FocusNeighbor(ctx, geometry.NeighborRight, geometry.NeighborOptions{}, *h.shortcutTarget()); err != nil {
		t.Fatalf("FocusNeighbor: %v", err)
	}
	front, found, err := h.windows.Frontmost(ctx)
	if err != nil || !found || front.PID != otherPID {
		t.Fatalf("frontmost = %+v (found %v, %v), want the window on the right at %s", front, found, err, right)
	}
}

func TestFocusNeighborWithoutAWindowInThatDirectionFails(t *testing.T) {
	h := newHarness(t)
	key := h.targetWindow(frameOf(0, 0, 400, 800))
	h.windows.Focus(key)

	err := h.service.FocusNeighbor(context.Background(), geometry.NeighborLeft, geometry.NeighborOptions{}, *h.shortcutTarget())
	requireFailure(t, err, ReasonNoNeighbor)
}
