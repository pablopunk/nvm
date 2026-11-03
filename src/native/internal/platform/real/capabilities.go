package real

import (
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Capabilities reports what this build supports on the running OS. MyGo implements the desktop intents
// on macOS, Windows and Linux; the verified matrix lives in the rig reports, not here. Process listing
// and signalling exist on Linux only until macOS and Windows get implementations.
func Capabilities() platform.Capabilities {
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
	if runtime.GOOS == "linux" {
		supported = append(supported, platform.CapabilityProcessList, platform.CapabilityProcessSignal)
	}
	if pasteSupported() {
		supported = append(supported, platform.CapabilityClipboardPaste)
	}
	return platform.NewCapabilities(supported...)
}
