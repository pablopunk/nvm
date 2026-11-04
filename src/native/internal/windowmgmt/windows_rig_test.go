//go:build rig

package windowmgmt

import (
	"context"
	"os"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// rigTolerance is how far a reached frame may differ from the requested one: window decorations and the
// window manager's rounding move edges by a few pixels on X11.
const rigTolerance = 4

// TestRigMovesAProbeWindowOnX11 drives the engine over the real X11 backend on the rig desktop. The rig
// script starts a window titled nvm-probe with ImageMagick's display; the test lays it out in halves,
// moves it to another display only when there is one, raises it, and restores it to where it began.
func TestRigMovesAProbeWindowOnX11(t *testing.T) {
	ports := real.NewPorts()
	if !ports.Capabilities.Supported(platform.CapabilityWindowManagement) {
		t.Skip("window management is not supported on this desktop")
	}
	service := NewService(Deps{Windows: ports.Windows, Clock: ports.Clock, SelfPID: os.Getpid()})
	ctx := context.Background()

	displays, err := service.ListDisplays(ctx)
	if err != nil || len(displays) == 0 {
		t.Fatalf("ListDisplays = %+v, %v", displays, err)
	}
	probes, err := service.ListWindows(ctx, WindowQuery{Title: "nvm-probe"})
	if err != nil || len(probes) != 1 {
		t.Fatalf("probe windows = %+v, %v; want exactly one nvm-probe window", probes, err)
	}
	probe := probes[0]
	work := displays[0].WorkArea
	left := mustRigResult(t, service, probe.Handle, "left-half")
	requireRigFrame(t, "left half", left.Frame, geometry.Rect{X: work.X, Y: work.Y, Width: work.Width / 2, Height: work.Height})

	right := mustRigResult(t, service, probe.Handle, "right-half")
	requireRigFrame(t, "right half", right.Frame, geometry.Rect{X: work.X + work.Width/2, Y: work.Y, Width: work.Width / 2, Height: work.Height})

	if err := service.FocusWindow(ctx, probe.Handle); err != nil {
		t.Fatalf("FocusWindow: %v", err)
	}
	restored := mustRigRestore(t, service, probe.Handle)
	requireRigFrame(t, "restored", restored.Frame, probe.Frame)
}

func mustRigResult(t *testing.T, service *Service, handle, preset string) Result {
	t.Helper()
	layout, ok := geometry.Preset(geometry.PresetID(preset))
	if !ok {
		t.Fatalf("preset %q is missing", preset)
	}
	result, err := service.ApplyToWindow(context.Background(), handle, layout, geometry.UniformGaps(0))
	if err != nil {
		t.Fatalf("%s: %v", preset, err)
	}
	t.Logf("%s reached %+v (%s)", preset, result.Frame, result.Outcome)
	return result
}

func mustRigRestore(t *testing.T, service *Service, handle string) Result {
	t.Helper()
	result, err := service.ApplyToWindow(context.Background(), handle, geometry.Layout{Kind: geometry.KindRestore}, geometry.UniformGaps(0))
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	return result
}

func requireRigFrame(t *testing.T, what string, got, want geometry.Rect) {
	t.Helper()
	if !rigClose(got.X, want.X) || !rigClose(got.Y, want.Y) || !rigClose(got.Width, want.Width) || !rigClose(got.Height, want.Height) {
		t.Fatalf("%s: reached %+v, want about %+v", what, got, want)
	}
}

func rigClose(a, b float64) bool {
	return a-b <= rigTolerance && b-a <= rigTolerance
}
