//go:build linux

package real

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const switchableProps = "_NET_WM_PID WM_CLASS _NET_WM_NAME WM_NAME _NET_WM_STATE _NET_WM_WINDOW_TYPE _NET_WM_ALLOWED_ACTIONS _NET_FRAME_EXTENTS"

// answerSwitchableWindow scripts one window: its properties and a 300x200 client rectangle.
func answerSwitchableWindow(desktop *scriptedDesktop, id, props string) {
	desktop.answer("xprop -id "+id+" "+switchableProps, props)
	desktop.answer("xwininfo -id "+id, nevermindGeometryOutput)
}

func windowPropsOf(pid int, class, title string, state, windowType string) string {
	return fmt.Sprintf(`_NET_WM_PID(CARDINAL) = %d
WM_CLASS(STRING) = "%s", "%s"
_NET_WM_NAME(UTF8_STRING) = "%s"
_NET_WM_STATE(ATOM) = %s
_NET_WM_WINDOW_TYPE(ATOM) = %s
`, pid, class, class, title, state, windowType)
}

// TestRunningAppsAndWindowsLeaveOutDocksDesktopsSkipTaskbarAndOwnProcess scripts a desktop with a panel, a desktop
// window, a taskbar-skipping helper, Nevermind's own window and two terminals; only the terminals are listed.
func TestRunningAppsAndWindowsLeaveOutDocksDesktopsSkipTaskbarAndOwnProcess(t *testing.T) {
	const (
		panelID, desktopID, helperID, ownID, terminalID, otherTerminalID = "0x300001", "0x300002", "0x300003", "0x300004", "0x300005", "0x300006"
		terminalPID, otherTerminalPID                                    = 55002, 55003
	)
	desktop := newScriptedDesktop()
	desktop.answer(monitorsCommand, monitorsOutput)
	desktop.answer(workAreaCommand, "_NET_WORKAREA(CARDINAL) = 0, 27, 1280, 773, 0, 27, 1280, 773")
	desktop.answer(stackingCommand, "_NET_CLIENT_LIST_STACKING(WINDOW): window id # "+panelID+", "+desktopID+", "+helperID+", "+ownID+", "+terminalID+", "+otherTerminalID)
	answerSwitchableWindow(desktop, panelID, windowPropsOf(91819, "xfce4-panel", "xfce4-panel", "_NET_WM_STATE_ABOVE", "_NET_WM_WINDOW_TYPE_DOCK"))
	answerSwitchableWindow(desktop, desktopID, windowPropsOf(91820, "xfdesktop", "Desktop", "", "_NET_WM_WINDOW_TYPE_DESKTOP"))
	answerSwitchableWindow(desktop, helperID, windowPropsOf(55001, "helper", "Helper", "_NET_WM_STATE_SKIP_TASKBAR", "_NET_WM_WINDOW_TYPE_NORMAL"))
	answerSwitchableWindow(desktop, ownID, windowPropsOf(os.Getpid(), "Nvm-clone", "Nevermind", "", "_NET_WM_WINDOW_TYPE_NORMAL"))
	answerSwitchableWindow(desktop, terminalID, windowPropsOf(terminalPID, "Terminal", "~", "", "_NET_WM_WINDOW_TYPE_NORMAL"))
	answerSwitchableWindow(desktop, otherTerminalID, windowPropsOf(otherTerminalPID, "Terminal", "~", "", "_NET_WM_WINDOW_TYPE_NORMAL"))
	windows := x11Windows{helper: desktop}

	apps, err := windows.RunningApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := pidsOfApps(apps); !equalPIDs(got, []int{otherTerminalPID, terminalPID}) {
		t.Fatalf("RunningApps = %v, want the two terminals front to back", got)
	}
	listed, err := windows.AllWindows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Title != "~" || listed[1].Title != "~" {
		t.Fatalf("AllWindows = %+v, want the two terminal windows", listed)
	}
}

func TestOnlyAppWindowsAreSwitchable(t *testing.T) {
	const self = 4242
	tests := []struct {
		name   string
		window x11Window
		want   bool
	}{
		{"an app window", x11Window{PID: 7}, true},
		{"Nevermind's own window", x11Window{PID: self}, false},
		{"a dock or panel", x11Window{PID: 7, Shell: true}, false},
		{"a skip-taskbar window", x11Window{PID: 7, SkipTaskbar: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.window.isSwitchableWindow(self); got != tt.want {
				t.Fatalf("isSwitchableWindow = %v, want %v", got, tt.want)
			}
		})
	}
}

func pidsOfApps(apps []platform.WindowApp) []int {
	pids := make([]int, 0, len(apps))
	for _, app := range apps {
		pids = append(pids, app.PID)
	}
	return pids
}

func equalPIDs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
