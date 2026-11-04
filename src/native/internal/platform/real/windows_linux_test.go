package real

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// scriptedDesktop answers X11 tool calls from queues keyed by the exact command line. The last answer of a
// queue repeats, so a window read before and after a write can answer differently.
type scriptedDesktop struct {
	mu      sync.Mutex
	answers map[string][]platform.HelperResult
	calls   []string
}

func newScriptedDesktop() *scriptedDesktop {
	return &scriptedDesktop{answers: map[string][]platform.HelperResult{}}
}

func (d *scriptedDesktop) answer(command string, stdout string) {
	d.queue(command, platform.HelperResult{Stdout: []byte(stdout)})
}

func (d *scriptedDesktop) fail(command string) {
	d.mu.Lock()
	delete(d.answers, command)
	d.mu.Unlock()
	d.queue(command, platform.HelperResult{ExitCode: 1, Stderr: []byte("X Error")})
}

func (d *scriptedDesktop) queue(command string, result platform.HelperResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.answers[command] = append(d.answers[command], result)
}

func (d *scriptedDesktop) Run(_ context.Context, binary string, args []string, _ []byte) (platform.HelperResult, error) {
	command := strings.Join(append([]string{binary}, args...), " ")
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, command)
	queue := d.answers[command]
	if len(queue) == 0 {
		return platform.HelperResult{ExitCode: 127, Stderr: []byte("unscripted: " + command)}, nil
	}
	if len(queue) > 1 {
		d.answers[command] = queue[1:]
	}
	return queue[0], nil
}

func (d *scriptedDesktop) called(command string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, call := range d.calls {
		if call == command {
			return true
		}
	}
	return false
}

const (
	nevermindID       = "0x400010"
	panelID           = "0xa00003"
	stackingCommand   = "xprop -root _NET_CLIENT_LIST_STACKING"
	nevermindProps    = "xprop -id " + nevermindID + " _NET_WM_PID WM_CLASS _NET_WM_NAME WM_NAME _NET_WM_STATE _NET_WM_ALLOWED_ACTIONS _NET_FRAME_EXTENTS"
	nevermindGeometry = "xwininfo -id " + nevermindID
	panelProps        = "xprop -id " + panelID + " _NET_WM_PID WM_CLASS _NET_WM_NAME WM_NAME _NET_WM_STATE _NET_WM_ALLOWED_ACTIONS _NET_FRAME_EXTENTS"
	panelGeometry     = "xwininfo -id " + panelID
	monitorsCommand   = "xrandr --query"
	workAreaCommand   = "xprop -root _NET_WORKAREA"
)

const (
	nevermindPropsOutput = `_NET_WM_PID(CARDINAL) = 99763
WM_CLASS(STRING) = "nvm-clone", "Nvm-clone"
_NET_WM_NAME(UTF8_STRING) = "Nevermind"
WM_NAME(STRING) = "Nevermind"
_NET_WM_STATE(ATOM) = _NET_WM_STATE_SKIP_TASKBAR, _NET_WM_STATE_ABOVE
_NET_WM_ALLOWED_ACTIONS(ATOM) = _NET_WM_ACTION_MOVE, _NET_WM_ACTION_RESIZE
`
	panelPropsOutput = `_NET_WM_PID(CARDINAL) = 91819
WM_CLASS(STRING) = "xfce4-panel", "Xfce4-panel"
_NET_WM_NAME(UTF8_STRING) = "xfce4-panel"
`
	nevermindGeometryOutput = `  Absolute upper-left X:  280
  Absolute upper-left Y:  128
  Width: 720
  Height: 470
`
	reachedGeometryOutput = `  Absolute upper-left X:  0
  Absolute upper-left Y:  0
  Width: 500
  Height: 800
`
	panelGeometryOutput = `  Absolute upper-left X:  0
  Absolute upper-left Y:  0
  Width: 1280
  Height: 27
`
	monitorsOutput = "Screen 0: minimum 1 x 1, current 1280 x 800, maximum 1280 x 800\nscreen connected 1280x800+0+0 0mm x 0mm\n"
)

func newTestDesktop(t *testing.T) (x11Windows, *scriptedDesktop) {
	t.Helper()
	desktop := newScriptedDesktop()
	desktop.answer(monitorsCommand, monitorsOutput)
	desktop.answer(workAreaCommand, "_NET_WORKAREA(CARDINAL) = 0, 27, 1280, 773, 0, 27, 1280, 773")
	desktop.answer(stackingCommand, "_NET_CLIENT_LIST_STACKING(WINDOW): window id # 0xa00003, 0x400010")
	desktop.answer(panelProps, panelPropsOutput)
	desktop.answer(panelGeometry, panelGeometryOutput)
	desktop.answer(nevermindProps, nevermindPropsOutput)
	desktop.answer(nevermindGeometry, nevermindGeometryOutput)
	return x11Windows{helper: desktop}, desktop
}

func TestDisplaysUseTheMonitorIntersectedWithTheDesktopWorkArea(t *testing.T) {
	windows, _ := newTestDesktop(t)

	displays, err := windows.Displays(context.Background())
	if err != nil || len(displays) != 1 {
		t.Fatalf("Displays = %+v, %v", displays, err)
	}
	display := displays[0]
	if !display.Primary || display.Frame.Height != 800 || display.WorkArea != (platform.WindowFrame{X: 0, Y: 27, Width: 1280, Height: 773}) {
		t.Fatalf("display = %+v", display)
	}
}

func TestWindowsOfAProcessCarryFramesDisplayAndStackOrder(t *testing.T) {
	windows, _ := newTestDesktop(t)

	listed, err := windows.Windows(context.Background(), 99763)
	if err != nil || len(listed) != 1 {
		t.Fatalf("Windows(99763) = %+v, %v", listed, err)
	}
	record := listed[0]
	if record.Key != "99763:4194320" || record.Title != "Nevermind" || record.BundleID != "Nvm-clone" {
		t.Fatalf("record identity = %+v", record)
	}
	if record.Frame != (platform.WindowFrame{X: 280, Y: 128, Width: 720, Height: 470}) || record.DisplayID != "screen" || record.ZIndex != 0 {
		t.Fatalf("record geometry = %+v", record)
	}
	if !record.Resizable || !record.Movable || record.Minimized {
		t.Fatalf("record capabilities = %+v", record)
	}
}

func TestFrontmostIsTheOwnerOfTheActiveWindow(t *testing.T) {
	windows, desktop := newTestDesktop(t)
	desktop.answer("xprop -root _NET_ACTIVE_WINDOW", "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x400010")

	app, found, err := windows.Frontmost(context.Background())
	if err != nil || !found || app.PID != 99763 || app.BundleID != "Nvm-clone" {
		t.Fatalf("Frontmost = %+v, %v, %v", app, found, err)
	}
}

func TestSetFrameMovesAndResizesThenReadsBackWhatTheWindowReached(t *testing.T) {
	windows, desktop := newTestDesktop(t)
	desktop.answer(nevermindGeometry, reachedGeometryOutput)
	key := "99763:4194320"
	desktop.answer("xdotool windowmove --sync "+nevermindID+" 0 0", "")
	desktop.answer("xdotool windowsize --sync "+nevermindID+" 500 800", "")

	reached, err := windows.SetFrame(context.Background(), platform.WindowWrite{PID: 99763, Key: key, Frame: platform.WindowFrame{Width: 500, Height: 800}})
	if err != nil {
		t.Fatalf("SetFrame: %v", err)
	}
	if reached != (platform.WindowFrame{Width: 500, Height: 800}) {
		t.Fatalf("reached = %+v", reached)
	}
	if !desktop.called("xdotool windowmove --sync " + nevermindID + " 0 0") || !desktop.called("xdotool windowsize --sync " + nevermindID + " 500 800") {
		t.Fatalf("move or resize was not issued: %v", desktop.calls)
	}
}

func TestSetFrameMovesTheFrameOriginAndSizesTheClientInsideDecorations(t *testing.T) {
	desktop := newScriptedDesktop()
	desktop.answer(nevermindProps, nevermindPropsOutput+"_NET_FRAME_EXTENTS(CARDINAL) = 1, 1, 20, 5\n")
	desktop.answer(nevermindGeometry, nevermindGeometryOutput)
	desktop.answer("xdotool windowmove --sync "+nevermindID+" 0 27", "")
	desktop.answer("xdotool windowsize --sync "+nevermindID+" 498 775", "")
	desktop.answer(nevermindGeometry, "  Absolute upper-left X:  1\n  Absolute upper-left Y:  47\n  Width: 498\n  Height: 775\n")
	windows := x11Windows{helper: desktop}

	reached, err := windows.SetFrame(context.Background(), platform.WindowWrite{PID: 99763, Key: "99763:4194320", Frame: platform.WindowFrame{Y: 27, Width: 500, Height: 800}})
	if err != nil {
		t.Fatalf("SetFrame: %v", err)
	}
	if !desktop.called("xdotool windowmove --sync "+nevermindID+" 0 27") || !desktop.called("xdotool windowsize --sync "+nevermindID+" 498 775") {
		t.Fatalf("decorated move used the wrong coordinates: %v", desktop.calls)
	}
	if reached != (platform.WindowFrame{X: 0, Y: 27, Width: 500, Height: 800}) {
		t.Fatalf("reached = %+v, want the requested frame read back through the decorations", reached)
	}
}

func TestSetFrameOfAVanishedWindowIsTargetLost(t *testing.T) {
	windows, desktop := newTestDesktop(t)
	desktop.fail(nevermindProps)

	_, err := windows.SetFrame(context.Background(), platform.WindowWrite{PID: 99763, Key: "99763:4194320"})
	requireWindowCode(t, err, platform.WindowErrorTargetLost)
	if desktop.called("xdotool windowmove --sync " + nevermindID + " 0 0") {
		t.Fatal("a window that vanished was still moved")
	}
}

func TestSetFrameRefusesAWindowOwnedByAnotherProcess(t *testing.T) {
	windows, _ := newTestDesktop(t)

	_, err := windows.SetFrame(context.Background(), platform.WindowWrite{PID: 1, Key: "99763:4194320"})
	requireWindowCode(t, err, platform.WindowErrorTargetLost)
}

func TestFullScreenIsReportedUnsupportedWithoutTouchingTheWindow(t *testing.T) {
	windows, desktop := newTestDesktop(t)

	_, err := windows.SetFullScreen(context.Background(), 99763, "99763:4194320", true)
	requireWindowCode(t, err, platform.WindowErrorUnsupported)
	for _, call := range desktop.calls {
		if strings.HasPrefix(call, "xdotool") {
			t.Fatalf("full screen issued %q", call)
		}
	}
}

func TestAccessIsUnknownWhenTheDesktopDoesNotAnswerEWMH(t *testing.T) {
	windows := x11Windows{helper: newScriptedDesktop()}

	access, err := windows.Access(context.Background())
	if err != nil || access != platform.WindowAccessUnknown {
		t.Fatalf("Access = %v, %v; want unknown", access, err)
	}
}

func requireWindowCode(t *testing.T, err error, code platform.WindowErrorCode) {
	t.Helper()
	var windowErr *platform.WindowError
	if !errors.As(err, &windowErr) || windowErr.Code != code {
		t.Fatalf("error = %v, want window error %q", err, code)
	}
}
