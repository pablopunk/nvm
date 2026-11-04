package real

import (
	"os"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Capabilities reports what this build supports on the running OS. MyGo implements the desktop intents
// on macOS, Windows and Linux; the verified matrix lives in the rig reports, not here. Process listing
// and signalling exist on Linux only until macOS and Windows get implementations.
func Capabilities() platform.Capabilities {
	return capabilitiesFor(runtime.GOOS, os.Getenv)
}

// capabilitiesFor is Capabilities for a named OS and environment, so the session rules are testable.
func capabilitiesFor(goos string, getenv func(string) string) platform.Capabilities {
	supported := []platform.Capability{
		platform.CapabilityAutostart,
		platform.CapabilityClipboardChanged,
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
	if !isWaylandSession(goos, getenv) {
		supported = append(supported, platform.CapabilityWindowPlacement)
	}
	return platform.NewCapabilities(supported...)
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
