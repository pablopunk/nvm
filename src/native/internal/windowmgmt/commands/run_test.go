package commands

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

const (
	targetPID         = 42
	otherPID          = 77
	selfPID           = 9
	copyNoTarget      = "No target app was captured. Reopen Nevermind from the app you want to move, then try again."
	unknownHandleCopy = "That window handle is no longer valid. List windows again."
)

// desk is a 1000 by 800 display with one focused target window and an optional neighbor.
type desk struct {
	windows *fake.WindowManager
	runner  Runner
	target  windowmgmt.Target
	key     string
}

func newDesk(t *testing.T, settings geometry.Settings) desk {
	t.Helper()
	return newDeskWith(t, settings, func(*fake.SimWindow) {})
}

// newDeskWith is newDesk with the target window's spec adjusted by tune before it is added.
func newDeskWith(t *testing.T, settings geometry.Settings, tune func(*fake.SimWindow)) desk {
	t.Helper()
	windows := fake.NewWindowManager()
	windows.SetDisplays(platform.WindowDisplay{
		ID:       "main",
		Primary:  true,
		Frame:    platform.WindowFrame{Width: 1000, Height: 800},
		WorkArea: platform.WindowFrame{Width: 1000, Height: 800},
	})
	spec := fake.SimWindow{PID: targetPID, BundleID: "app.target", AppName: "Target",
		Title: "Document", Frame: platform.WindowFrame{X: 100, Y: 100, Width: 300, Height: 300}}
	tune(&spec)
	key := windows.AddWindow(spec)
	windows.Focus(key)
	service := windowmgmt.NewService(windowmgmt.Deps{Windows: windows, Clock: fake.NewClock(), SelfPID: selfPID})
	return desk{
		windows: windows,
		runner:  Runner{Engine: service, Settings: func() geometry.Settings { return settings }},
		target:  windowmgmt.Target{PID: targetPID, BundleID: "app.target"},
		key:     key,
	}
}

func shortcutFrom(target windowmgmt.Target) windowmgmt.Invocation {
	return windowmgmt.Invocation{Source: windowmgmt.SourceShortcut, Target: &target}
}

func mustCommand(t *testing.T, id string) Command {
	t.Helper()
	command, ok := ByID(id)
	if !ok {
		t.Fatalf("no command %q", id)
	}
	return command
}

func TestExecuteAppliesLayoutToTheFocusedWindow(t *testing.T) {
	d := newDesk(t, geometry.DefaultSettings())
	message := d.runner.Execute(context.Background(), mustCommand(t, "window.left-half"), shortcutFrom(d.target))
	if message != "" {
		t.Fatalf("message = %q, want silent success", message)
	}
	window, _ := d.windows.Window(d.key)
	if window.Frame.X != 8 || window.Frame.Y != 8 {
		t.Errorf("origin = (%v, %v), want the outer gap (8, 8)", window.Frame.X, window.Frame.Y)
	}
	if window.Frame.Width != 490 {
		t.Errorf("width = %v, want half of the work area less the gaps (490)", window.Frame.Width)
	}
}

func TestExecuteUsesTheUsersGapSettings(t *testing.T) {
	settings := geometry.DefaultSettings()
	settings.OuterGap = 0
	settings.InnerGap = 0
	d := newDesk(t, settings)
	d.runner.Execute(context.Background(), mustCommand(t, "window.left-half"), shortcutFrom(d.target))
	window, _ := d.windows.Window(d.key)
	if window.Frame.X != 0 || window.Frame.Width != 500 {
		t.Errorf("frame = %+v, want x 0 and width 500 with no gaps", window.Frame)
	}
}

func TestExecuteWithoutATargetExplainsThePalette(t *testing.T) {
	d := newDesk(t, geometry.DefaultSettings())
	message := d.runner.Execute(context.Background(), mustCommand(t, "window.left-half"),
		windowmgmt.Invocation{Source: windowmgmt.SourcePalette})
	if message != copyNoTarget {
		t.Errorf("message = %q", message)
	}
}

func TestExecuteReportsAPartialWithItsMessage(t *testing.T) {
	d := newDeskWith(t, geometry.DefaultSettings(), func(spec *fake.SimWindow) { spec.MinWidth = 900 })
	message := d.runner.Execute(context.Background(), mustCommand(t, "window.left-half"), shortcutFrom(d.target))
	if message != geometry.MessagePartialGeneric {
		t.Errorf("message = %q, want the partial copy", message)
	}
}

func TestFocusCommandRaisesTheNeighborWindow(t *testing.T) {
	d := newDesk(t, geometry.DefaultSettings())
	neighbor := d.windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: "app.other", AppName: "Other",
		Title: "Terminal", Frame: platform.WindowFrame{X: 600, Y: 100, Width: 300, Height: 300}})
	d.windows.Focus(d.key)
	message := d.runner.Execute(context.Background(), mustCommand(t, "window.focus-right"), shortcutFrom(d.target))
	if message != "" {
		t.Fatalf("message = %q", message)
	}
	front, found, _ := d.windows.Frontmost(context.Background())
	if !found || front.PID != otherPID {
		t.Errorf("front app = %+v, want the neighbor %q", front, neighbor)
	}
}

func TestExecuteOnWindowWithAnUnknownHandleSaysSo(t *testing.T) {
	d := newDesk(t, geometry.DefaultSettings())
	message := d.runner.ExecuteOnWindow(context.Background(), mustCommand(t, "window.maximize"), "no-such-handle")
	if message != unknownHandleCopy {
		t.Errorf("message = %q, want the unknown handle copy", message)
	}
}
