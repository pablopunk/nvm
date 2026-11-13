//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newOutputMute returns the macOS output mute, which runs AppleScript through the helper's osascript.
func newOutputMute(helper platform.Helper) platform.OutputMuter {
	return osascriptOutputMute{helper: helper}
}
