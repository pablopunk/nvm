package real

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Capabilities reports what this build supports on the running OS. MyGo implements the desktop intents
// on macOS, Windows and Linux; the verified matrix lives in the rig reports, not here. Process listing
// and signalling exist on Linux only until macOS and Windows get implementations.
func Capabilities() platform.Capabilities {
	return withWebViewCapabilities(capabilitiesFor(runtime.GOOS, os.Getenv, hasTool), mygo.WebViewAvailable())
}

// withWebViewCapabilities adds the web view intents when the system has a web view. Camera preview runs on a web
// page, so it follows the web view and needs no separate OS check.
func withWebViewCapabilities(caps platform.Capabilities, webViewAvailable bool) platform.Capabilities {
	if !webViewAvailable {
		return caps
	}
	return platform.NewCapabilities(append(caps.List(), platform.CapabilityWebView, platform.CapabilityCameraPreview)...)
}

// hasTool reports whether an executable of that name is on the PATH.
func hasTool(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// capabilitiesFor is Capabilities for a named OS, environment and set of tools, so the session rules are testable.
func capabilitiesFor(goos string, getenv func(string) string, hasTool func(string) bool) platform.Capabilities {
	supported := []platform.Capability{
		platform.CapabilityAutostart,
		platform.CapabilityClipboardImage,
		platform.CapabilityFileWatch,
		platform.CapabilityGlobalHotkeys,
		platform.CapabilityHelperRun,
		platform.CapabilityKeychain,
		platform.CapabilityScreensDisplays,
		platform.CapabilityShellApps,
		platform.CapabilityShellAppIcons,
		platform.CapabilityShellBeep,
		platform.CapabilityShellReveal,
		platform.CapabilityShellTrash,
		platform.CapabilityTrayMenu,
	}
	if goos == "linux" {
		supported = append(supported, platform.CapabilityProcessList, platform.CapabilityProcessSignal, platform.CapabilityAppRunning, platform.CapabilityAppQuit, platform.CapabilityAppUninstall)
	}
	if goos == "darwin" {
		supported = append(supported, platform.CapabilityShellQuickLook, platform.CapabilityKeyboardTypeText, platform.CapabilityAppRunning, platform.CapabilityAppQuit, platform.CapabilityAppHide, platform.CapabilityAppUninstall)
	}
	if notificationsCapable(goos) {
		supported = append(supported, platform.CapabilityNotifications)
	}
	if pasteCapable(goos, getenv, hasTool) {
		supported = append(supported, platform.CapabilityClipboardPaste)
	}
	if selectionCopySupported(goos, getenv, hasTool) {
		supported = append(supported, platform.CapabilitySelectionCopy)
	}
	supported = append(supported, platform.CapabilityWindowAlwaysOnTop)
	if goos != "windows" {
		supported = append(supported, platform.CapabilityWindowAllSpaces)
	}
	if !isWaylandSession(goos, getenv) {
		supported = append(supported, platform.CapabilityWindowPlacement, platform.CapabilityWindowFrameRestore, platform.CapabilityWindowDisplayRecovery)
	}
	if windowManagementCapable(goos, getenv, hasTool) {
		supported = append(supported, platform.CapabilityWindowManagement)
	}
	if isX11FocusSession(goos, getenv) && hasTool("xdotool") {
		supported = append(supported, platform.CapabilityFocusReturn)
	}
	if frontmostAppCapable(goos, getenv, hasTool) {
		supported = append(supported, platform.CapabilityFrontmostApp)
	}
	env := processSystemEnvironment()
	env.goos = goos
	env.installed = hasTool
	supported = append(supported, systemCapabilities(env)...)
	if goos == "linux" && hasTool("pactl") && hasTool("parec") {
		supported = append(supported, platform.CapabilityMicrophone)
	}
	supported = append(supported, textCapabilities(ocrAssetsAvailable(), screenCaptureCapable(goos, getenv, hasTool))...)
	return platform.NewCapabilities(supported...)
}

// textCapabilities advertises recognition only when its bundled runtime and models are available.
func textCapabilities(hasOCR, canCaptureScreen bool) []platform.Capability {
	var supported []platform.Capability
	if hasOCR {
		supported = append(supported, platform.CapabilityTextRecognize)
	}
	if canCaptureScreen {
		supported = append(supported, platform.CapabilityScreenCapture)
	}
	return supported
}

// xdotoolBinary is the X11 input tool: it pastes, reads and restores the focused window, and places windows.
const xdotoolBinary = "xdotool"

// x11WindowTools are the X11 programs the window commands call: xprop and xwininfo read windows, xrandr lists the
// monitors and xdotool places and raises them. A missing one would fail every window command, so all four are needed.
var x11WindowTools = []string{"xprop", "xwininfo", "xrandr", xdotoolBinary}

// pasteCapable reports whether the paste gesture can run: System Events on macOS, and an X11 session with xdotool on
// Linux. Windows has no paste helper in this build, and a Wayland session has no key injection here.
func pasteCapable(goos string, getenv func(string) string, hasTool func(string) bool) bool {
	switch goos {
	case "darwin":
		return true
	case "linux":
		return isX11FocusSession(goos, getenv) && hasTool(xdotoolBinary)
	}
	return false
}

// windowManagementCapable reports whether other apps' windows can be moved. Only an X11 session can, because X11 tools
// see no native Wayland window even when XWayland sets DISPLAY, so a Wayland session is omitted regardless of DISPLAY.
func windowManagementCapable(goos string, getenv func(string) string, hasTool func(string) bool) bool {
	switch goos {
	case "darwin":
		return true
	case "linux":
		return isX11FocusSession(goos, getenv) && hasAllTools(hasTool, x11WindowTools)
	}
	return false
}

// frontmostAppCapable reports whether the app in front can be named: Launch Services on macOS, and on Linux the same
// X11 tools the window port reads the active window with. Windows has no port for it in this build.
func frontmostAppCapable(goos string, getenv func(string) string, hasTool func(string) bool) bool {
	switch goos {
	case "darwin":
		return true
	case "linux":
		return windowManagementCapable(goos, getenv, hasTool)
	}
	return false
}

// notificationsCapable is false on Windows: the 1.x desktop inventory has no
// notification surface, so the capability is omitted there rather than claimed through MyGo's untested toast path.
func notificationsCapable(goos string) bool {
	return goos != "windows"
}

// hasAllTools reports whether every named tool is installed.
func hasAllTools(hasTool func(string) bool, names []string) bool {
	for _, name := range names {
		if !hasTool(name) {
			return false
		}
	}
	return true
}

// isWaylandSession reports whether the desktop is a Wayland session on Linux. A Wayland client cannot place
// its own window, so the palette is left where the compositor puts it (Electron's
// supportsProgrammaticWindowPositioning).
func isWaylandSession(goos string, getenv func(string) string) bool {
	if goos != "linux" {
		return false
	}
	return getenv("XDG_SESSION_TYPE") == "wayland" || getenv("WAYLAND_DISPLAY") != ""
}

// isX11FocusSession reports whether the focus port can work: an X11 display on Linux, where xdotool reads and
// restores the focused window. Wayland and other systems have no such tool here.
func isX11FocusSession(goos string, getenv func(string) string) bool {
	return goos == "linux" && !isWaylandSession(goos, getenv) && getenv("DISPLAY") != ""
}

// selectionCopySupported reports whether the copy gesture can run: on Linux the X11 session with xdotool (the same
// rule as paste), on macOS always (the osascript gesture is marked UNTESTED until the L5 run), and nowhere else.
func selectionCopySupported(goos string, getenv func(string) string, hasTool func(string) bool) bool {
	switch goos {
	case "linux":
		return isX11FocusSession(goos, getenv) && hasTool("xdotool")
	case "darwin":
		return true
	}
	return false
}
