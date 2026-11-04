package windowmgmt

import (
	"context"
	"fmt"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

const (
	targetPID      = 42
	targetBundle   = "app.target"
	otherPID       = 77
	otherBundle    = "app.other"
	selfPIDForTest = 9
)

// harness is a simulated desktop wired to a Service: the palette focus host hands focus back by focusing
// the window named in restoreKey, as the real palette does.
type harness struct {
	t       *testing.T
	windows *fake.WindowManager
	clock   *fake.Clock
	focus   *stubFocus
	service *Service
}

type stubFocus struct {
	windows    *fake.WindowManager
	paletteApp *Target
	restoreKey string
	handedBack bool
}

func (f *stubFocus) PaletteFocusTarget() (Target, bool) {
	if f.paletteApp == nil {
		return Target{}, false
	}
	return *f.paletteApp, true
}

func (f *stubFocus) HidePaletteAndRestoreFocus(_ context.Context, _ Target) bool {
	f.handedBack = true
	if f.restoreKey != "" {
		f.windows.Focus(f.restoreKey)
	}
	return true
}

func newHarness(t *testing.T, displays ...platform.WindowDisplay) *harness {
	t.Helper()
	if len(displays) == 0 {
		displays = []platform.WindowDisplay{testDisplay("main", 0, 0, 1000, 800, true)}
	}
	windows := fake.NewWindowManager()
	windows.SetDisplays(displays...)
	clock := fake.NewClock()
	focus := &stubFocus{windows: windows}
	service := NewService(Deps{
		Windows:     windows,
		Focus:       focus,
		Clock:       clock,
		SelfPID:     selfPIDForTest,
		NewHandleID: sequentialHandles(),
	})
	return &harness{t: t, windows: windows, clock: clock, focus: focus, service: service}
}

func testDisplay(id string, x, y, width, height float64, primary bool) platform.WindowDisplay {
	frame := platform.WindowFrame{X: x, Y: y, Width: width, Height: height}
	return platform.WindowDisplay{ID: id, Primary: primary, Frame: frame, WorkArea: frame}
}

func sequentialHandles() func() string {
	next := 0
	return func() string {
		next++
		return fmt.Sprintf("handle-%d", next)
	}
}

// targetWindow adds a resizable window of the target app with the given frame and returns its key.
func (h *harness) targetWindow(frame platform.WindowFrame) string {
	h.t.Helper()
	return h.windows.AddWindow(fake.SimWindow{
		PID:      targetPID,
		BundleID: targetBundle,
		AppName:  "Target",
		Title:    "Document",
		Frame:    frame,
	})
}

func (h *harness) window(key string) fake.SimWindow {
	h.t.Helper()
	window, ok := h.windows.Window(key)
	if !ok {
		h.t.Fatalf("window %q is missing", key)
	}
	return window
}

// displayOf returns the display a target window currently sits on, as the window system reports it.
func (h *harness) displayOf(key string) string {
	h.t.Helper()
	records, err := h.windows.Windows(context.Background(), targetPID)
	if err != nil {
		h.t.Fatalf("Windows: %v", err)
	}
	for _, record := range records {
		if record.Key == key {
			return record.DisplayID
		}
	}
	h.t.Fatalf("window %q is not listed", key)
	return ""
}

func (h *harness) paletteTarget() *Invocation {
	return &Invocation{Source: SourcePalette, Target: &Target{PID: targetPID, BundleID: targetBundle}}
}

func (h *harness) shortcutTarget() *Invocation {
	return &Invocation{Source: SourceShortcut, Target: &Target{PID: targetPID, BundleID: targetBundle}}
}

// openedFromTarget makes the palette remember the target app, as it does when it opens over that app.
func (h *harness) openedFromTarget(restoreKey string) {
	h.focus.paletteApp = &Target{PID: targetPID, BundleID: targetBundle}
	h.focus.restoreKey = restoreKey
}

func frameOf(x, y, width, height float64) platform.WindowFrame {
	return platform.WindowFrame{X: x, Y: y, Width: width, Height: height}
}

func rectOf(x, y, width, height float64) geometry.Rect {
	return geometry.Rect{X: x, Y: y, Width: width, Height: height}
}

func presetLayout(t *testing.T, id geometry.PresetID) geometry.Layout {
	t.Helper()
	layout, ok := geometry.Preset(id)
	if !ok {
		t.Fatalf("preset %q is missing", id)
	}
	return layout
}

func requireFailure(t *testing.T, err error, want FailureReason) *Failure {
	t.Helper()
	failed, ok := err.(*Failure)
	if !ok {
		t.Fatalf("error = %v (%T), want *Failure with reason %q", err, err, want)
	}
	if failed.Reason != want {
		t.Fatalf("failure reason = %q (%q), want %q", failed.Reason, failed.Message, want)
	}
	return failed
}
