package real

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// linuxDesktopSession is one desktop a Linux build can start in. x11 is true when the X11 tools (xdotool, xprop,
// xwininfo, xrandr) can talk to the desktop; every Wayland session, XWayland included, is false.
type linuxDesktopSession struct {
	name string
	env  map[string]string
	x11  bool
}

var linuxDesktopMatrix = []linuxDesktopSession{
	{"GNOME on X11", map[string]string{"XDG_SESSION_TYPE": "x11", "XDG_CURRENT_DESKTOP": "GNOME", "DISPLAY": ":0"}, true},
	{"KDE Plasma on X11", map[string]string{"XDG_SESSION_TYPE": "x11", "XDG_CURRENT_DESKTOP": "KDE", "DISPLAY": ":0"}, true},
	{"XFCE on X11", map[string]string{"XDG_SESSION_TYPE": "x11", "XDG_CURRENT_DESKTOP": "XFCE", "DISPLAY": ":0"}, true},
	{"GNOME on Wayland", map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "GNOME", "WAYLAND_DISPLAY": "wayland-0"}, false},
	{"GNOME on Wayland with XWayland", map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "GNOME", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, false},
	{"KDE Plasma on Wayland", map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "KDE", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, false},
	{"Sway", map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "sway", "WAYLAND_DISPLAY": "wayland-1", "DISPLAY": ":0"}, false},
	{"Hyprland", map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "Hyprland", "WAYLAND_DISPLAY": "wayland-1"}, false},
}

// x11OnlyCapabilities are the frontmost-app intents that need X11 tools: they are offered on an X11 display and
// omitted on every Wayland session.
var x11OnlyCapabilities = []platform.Capability{
	platform.CapabilityClipboardPaste,
	platform.CapabilitySelectionCopy,
	platform.CapabilityFocusReturn,
	platform.CapabilityWindowPlacement,
	platform.CapabilityWindowManagement,
}

// sessionIndependentCapabilities are supported on every desktop session the matrix covers, through MyGo or the
// standard library: the portal-backed hotkeys and tray are declared here and fail with a palette message when the
// desktop lacks them (see docs in PLAN.md WP5.4).
var sessionIndependentCapabilities = []platform.Capability{
	platform.CapabilityAutostart,
	platform.CapabilityFileWatch,
	platform.CapabilityGlobalHotkeys,
	platform.CapabilityKeychain,
	platform.CapabilityProcessList,
	platform.CapabilityProcessSignal,
	platform.CapabilityScreensDisplays,
	platform.CapabilityTrayMenu,
}

func allLinuxToolsInstalled(string) bool { return true }

func TestLinuxDesktopMatrixOffersX11OnlyIntentsOnlyOnX11(t *testing.T) {
	for _, session := range linuxDesktopMatrix {
		t.Run(session.name, func(t *testing.T) {
			set := capabilitiesFor("linux", environment(session.env), allLinuxToolsInstalled)
			for _, capability := range x11OnlyCapabilities {
				if got := set.Supported(capability); got != session.x11 {
					t.Fatalf("%s supported = %v, want %v", capability, got, session.x11)
				}
			}
		})
	}
}

func TestLinuxDesktopMatrixKeepsSessionIndependentIntents(t *testing.T) {
	for _, session := range linuxDesktopMatrix {
		t.Run(session.name, func(t *testing.T) {
			set := capabilitiesFor("linux", environment(session.env), allLinuxToolsInstalled)
			for _, capability := range sessionIndependentCapabilities {
				if !set.Supported(capability) {
					t.Fatalf("%s is omitted, want it offered and left to the desktop to refuse", capability)
				}
			}
		})
	}
}

// TestXWaylandWindowManagementStaysOmitted is the regression for a session with DISPLAY set by XWayland: X11 tools
// reach only the XWayland windows, so the window commands must not be offered.
func TestXWaylandWindowManagementStaysOmitted(t *testing.T) {
	env := environment(map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"})
	if windowManagementCapable("linux", env, allLinuxToolsInstalled) {
		t.Fatal("window management is offered on a Wayland session with XWayland")
	}
}

func TestX11WindowManagementNeedsEveryX11Tool(t *testing.T) {
	x11 := environment(map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"})
	for _, missing := range x11WindowTools {
		hasTool := func(name string) bool { return name != missing }
		if windowManagementCapable("linux", x11, hasTool) {
			t.Fatalf("window management is offered without %s", missing)
		}
	}
	if !windowManagementCapable("linux", x11, allLinuxToolsInstalled) {
		t.Fatal("window management is omitted on X11 with every tool installed")
	}
}

func TestPasteAndSelectionNeedXdotoolOnX11(t *testing.T) {
	x11 := environment(map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"})
	withoutXdotool := func(name string) bool { return name != xdotoolBinary }
	if pasteCapable("linux", x11, withoutXdotool) || selectionCopySupported("linux", x11, withoutXdotool) {
		t.Fatal("paste or selection copy is offered without xdotool")
	}
	if !pasteCapable("linux", x11, allLinuxToolsInstalled) || !selectionCopySupported("linux", x11, allLinuxToolsInstalled) {
		t.Fatal("paste or selection copy is omitted on X11 with xdotool")
	}
}
