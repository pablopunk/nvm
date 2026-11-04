package real

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Capabilities reports what this build supports on the running OS. MyGo implements the desktop intents
// on macOS, Windows and Linux; the verified matrix lives in the rig reports, not here. Process listing
// and signalling exist on Linux only until macOS and Windows get implementations.
func Capabilities() platform.Capabilities {
	return capabilitiesFor(runtime.GOOS, os.Getenv, hasTool)
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
		platform.CapabilityNotifications,
		platform.CapabilityScreensDisplays,
		platform.CapabilityShellApps,
		platform.CapabilityShellAppIcons,
		platform.CapabilityShellBeep,
		platform.CapabilityShellReveal,
		platform.CapabilityShellTrash,
		platform.CapabilityTrayMenu,
	}
	if goos == "linux" {
		supported = append(supported, platform.CapabilityProcessList, platform.CapabilityProcessSignal)
	}
	if pasteSupported() {
		supported = append(supported, platform.CapabilityClipboardPaste)
	}
	if !isWaylandSession(goos, getenv) {
		supported = append(supported, platform.CapabilityWindowPlacement)
	}
	if windowManagementSupported() {
		supported = append(supported, platform.CapabilityWindowManagement)
	}
	if isX11FocusSession(goos, getenv) && hasTool("xdotool") {
		supported = append(supported, platform.CapabilityFocusReturn)
	}
	env := processSystemEnvironment()
	env.goos = goos
	supported = append(supported, systemCapabilities(env)...)
	if goos == "linux" && hasTool("pactl") && hasTool("parec") {
		supported = append(supported, platform.CapabilityMicrophone)
	}
	supported = append(supported, textCapabilities(goos, hasTool)...)
	return platform.NewCapabilities(supported...)
}

// textCapabilities lists the text intents of an OS. Reading text is on macOS (the Vision framework) and on Linux when
// Tesseract is installed; the screen can be captured only on macOS, so screen text is macOS only.
func textCapabilities(goos string, hasTool func(string) bool) []platform.Capability {
	var supported []platform.Capability
	if goos == "darwin" || (goos == "linux" && hasTool(tesseractBinary)) {
		supported = append(supported, platform.CapabilityTextRecognize)
	}
	if goos == "darwin" {
		supported = append(supported, platform.CapabilityScreenCapture)
	}
	return supported
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

// isX11FocusSession reports whether the focus port can work: an X11 session on Linux, where xdotool reads and
// restores the focused window. Wayland and other systems have no such tool here.
func isX11FocusSession(goos string, getenv func(string) string) bool {
	return goos == "linux" && !isWaylandSession(goos, getenv)
}
