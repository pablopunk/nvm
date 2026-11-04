package apps

import (
	"errors"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// A desktop entry for a GUI app, and the compositor, whose process matches a candidate but owns no window.
const (
	terminalEntry    = "[Desktop Entry]\nName=Terminal\nExec=/usr/bin/xterm-like --new\n"
	compositorEntry  = "[Desktop Entry]\nName=Compositor\nExec=picom --config x\n"
	terminalDesktop  = "/usr/share/applications/terminal.desktop"
	compositorDesk   = "/usr/share/applications/picom.desktop"
	terminalPID      = 301
	compositorPID    = 302
	unrelatedPIDName = "bash"
)

func guiRunningFixture(t *testing.T, guiPIDs func() (map[int]bool, error)) map[string]bool {
	t.Helper()
	files := fake.NewFS()
	files.Seed(terminalDesktop, []byte(terminalEntry))
	files.Seed(compositorDesk, []byte(compositorEntry))
	running := NewRunning(RunningOptions{
		Candidates: func() []App {
			return []App{{Name: "Terminal", Path: terminalDesktop}, {Name: "Compositor", Path: compositorDesk}}
		},
		Clock: fake.NewClock(),
		Processes: fake.NewProcesses(
			platform.Process{PID: terminalPID, Name: "xterm-like"},
			platform.Process{PID: compositorPID, Name: "picom"},
			platform.Process{PID: 303, Name: unrelatedPIDName},
		),
		Files:    files,
		Platform: "linux",
		GUIPIDs:  guiPIDs,
	})
	detected, err := running.detectWithProcesses(running.opts.Candidates())
	if err != nil {
		t.Fatal(err)
	}
	return detected
}

// TestOnlyAppsThatOwnAWindowCountAsRunningOnLinux keeps the compositor, which matches an app in the index but has no
// window, out of the running set; the app with a window stays.
func TestOnlyAppsThatOwnAWindowCountAsRunningOnLinux(t *testing.T) {
	got := guiRunningFixture(t, func() (map[int]bool, error) {
		return map[int]bool{terminalPID: true}, nil
	})
	want := map[string]bool{terminalDesktop: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("running = %v, want %v", got, want)
	}
}

func TestRunningKeepsEveryMatchWhenTheDesktopCannotListWindows(t *testing.T) {
	got := guiRunningFixture(t, func() (map[int]bool, error) {
		return nil, platform.ErrUnsupported
	})
	if !got[terminalDesktop] || !got[compositorDesk] {
		t.Fatalf("running = %v, want both apps when windows are unsupported", got)
	}
}

func TestRunningReportsAWindowListingFailure(t *testing.T) {
	failure := errors.New("xprop went away")
	files := fake.NewFS()
	running := NewRunning(RunningOptions{
		Clock:     fake.NewClock(),
		Processes: fake.NewProcesses(platform.Process{PID: terminalPID, Name: "xterm-like"}),
		Files:     files,
		GUIPIDs:   func() (map[int]bool, error) { return nil, failure },
	})
	if _, err := running.detectWithProcesses([]App{{Name: "Terminal", Path: terminalDesktop}}); !errors.Is(err, failure) {
		t.Fatalf("error = %v, want the window listing failure", err)
	}
}
