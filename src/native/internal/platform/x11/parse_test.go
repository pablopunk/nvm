package x11

import (
	"reflect"
	"testing"
)

// Outputs below were recorded from xprop, xwininfo and xrandr on the shared test box (Xvfb with openbox).

const recordedXpropTerminal = `_NET_WM_PID(CARDINAL) = 99763
WM_CLASS(STRING) = "nvm-clone", "Nvm-clone"
_NET_WM_NAME(UTF8_STRING) = "Nevermind"
WM_NAME(STRING) = "Nevermind"
_NET_WM_STATE(ATOM) = _NET_WM_STATE_SKIP_TASKBAR, _NET_WM_STATE_SKIP_PAGER, _NET_WM_STATE_ABOVE
_NET_WM_ALLOWED_ACTIONS(ATOM) = _NET_WM_ACTION_CHANGE_DESKTOP, _NET_WM_ACTION_CLOSE, _NET_WM_ACTION_MOVE
`

const recordedXpropPanel = `_NET_WM_PID(CARDINAL) = 91819
WM_CLASS(STRING) = "xfce4-panel", "Xfce4-panel"
_NET_WM_NAME(UTF8_STRING) = "xfce4-panel"
_NET_WM_STATE(ATOM) = 
_NET_WM_ALLOWED_ACTIONS(ATOM) = _NET_WM_ACTION_CHANGE_DESKTOP
_NET_WM_DESKTOP: not found.
`

const recordedXwininfo = `xwininfo: Window id: 0x400010 "Nevermind"

  Absolute upper-left X:  280
  Absolute upper-left Y:  128
  Relative upper-left X:  0
  Width: 720
  Height: 470
  Border width: 0
  Map State: IsViewable
`

const recordedXrandr = `Screen 0: minimum 1 x 1, current 1280 x 800, maximum 1280 x 800
screen connected 1280x800+0+0 0mm x 0mm
   1280x800       0.00* 
`

func TestPropertiesDecodeNamesWindowClassTitleAndAtoms(t *testing.T) {
	properties := ParseProperties(recordedXpropTerminal)

	if pid, ok := properties.Cardinal("_NET_WM_PID"); !ok || pid != 99763 {
		t.Fatalf("pid = %d, %v", pid, ok)
	}
	if got := properties.WMClass(); got != "Nvm-clone" {
		t.Fatalf("WMClass = %q, want the class (second string)", got)
	}
	if got := properties.String("_NET_WM_NAME"); got != "Nevermind" {
		t.Fatalf("title = %q", got)
	}
	if !properties.HasAtom("_NET_WM_STATE", "_NET_WM_STATE_ABOVE") {
		t.Fatal("state atom missing")
	}
	if properties.HasAtom("_NET_WM_STATE", "_NET_WM_STATE_FULLSCREEN") {
		t.Fatal("unexpected full-screen state")
	}
}

func TestUnsetAndMissingPropertiesReadAsEmpty(t *testing.T) {
	properties := ParseProperties(recordedXpropPanel)

	if got := properties.Atoms("_NET_WM_STATE"); got != nil {
		t.Fatalf("empty atom list = %v, want nil", got)
	}
	if _, ok := properties.Cardinal("_NET_WM_DESKTOP"); ok {
		t.Fatal("missing cardinal reported as present")
	}
	if got := properties.String("WM_NAME"); got != "" {
		t.Fatalf("absent string = %q", got)
	}
}

func TestEscapedQuotesInTitlesAreDecoded(t *testing.T) {
	properties := ParseProperties(`_NET_WM_NAME(UTF8_STRING) = "say \"hi\""`)
	if got := properties.String("_NET_WM_NAME"); got != `say "hi"` {
		t.Fatalf("title = %q", got)
	}
}

func TestWindowIDsReadTheStackingListAndTheActiveWindow(t *testing.T) {
	stacking := "_NET_CLIENT_LIST_STACKING(WINDOW): window id # 0xa00003, 0xa0000e, 0x400010"
	want := []uint32{0xa00003, 0xa0000e, 0x400010}
	if got := WindowIDs(stacking); !reflect.DeepEqual(got, want) {
		t.Fatalf("WindowIDs = %x, want %x", got, want)
	}
	if got := WindowIDs("_NET_ACTIVE_WINDOW: not found."); got != nil {
		t.Fatalf("unset list = %v, want nil", got)
	}
	if got := WindowIDs("_NET_ACTIVE_WINDOW(WINDOW): window id # 0x0"); !reflect.DeepEqual(got, []uint32{0}) {
		t.Fatalf("no active window = %v, want [0]", got)
	}
}

func TestWorkAreaUsesTheFirstDesktop(t *testing.T) {
	area, ok := WorkArea("_NET_WORKAREA(CARDINAL) = 0, 27, 1280, 773, 0, 27, 1280, 773")
	if !ok || area != (Rect{X: 0, Y: 27, Width: 1280, Height: 773}) {
		t.Fatalf("WorkArea = %+v, %v", area, ok)
	}
}

func TestGeometryReadsAbsoluteCornerAndSize(t *testing.T) {
	geometry, ok := Geometry(recordedXwininfo)
	if !ok || geometry != (Rect{X: 280, Y: 128, Width: 720, Height: 470}) {
		t.Fatalf("Geometry = %+v, %v", geometry, ok)
	}
	if _, ok := Geometry("xwininfo: no window"); ok {
		t.Fatal("Geometry accepted output without fields")
	}
}

func TestMonitorsReadConnectedOutputsWithPrimaryFlag(t *testing.T) {
	monitors := Monitors(recordedXrandr + "HDMI-1 connected primary 1920x1080+0+0 (normal)\nDP-2 connected 1280x1024-1920+0 (normal)\nVGA-1 disconnected (normal)\n")
	want := []Monitor{
		{Name: "screen", Bounds: Rect{Width: 1280, Height: 800}},
		{Name: "HDMI-1", Primary: true, Bounds: Rect{Width: 1920, Height: 1080}},
		{Name: "DP-2", Bounds: Rect{X: -1920, Width: 1280, Height: 1024}},
	}
	if !reflect.DeepEqual(monitors, want) {
		t.Fatalf("Monitors = %+v, want %+v", monitors, want)
	}
}

func TestIntersectReturnsTheOverlapOnly(t *testing.T) {
	overlap, ok := Rect{Width: 100, Height: 100}.Intersect(Rect{X: 50, Y: 50, Width: 100, Height: 100})
	if !ok || overlap != (Rect{X: 50, Y: 50, Width: 50, Height: 50}) {
		t.Fatalf("Intersect = %+v, %v", overlap, ok)
	}
	if _, ok := (Rect{Width: 10, Height: 10}).Intersect(Rect{X: 20, Width: 10, Height: 10}); ok {
		t.Fatal("disjoint rectangles intersect")
	}
}
