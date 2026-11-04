//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newMicrophone returns the macOS capture port, which records through the Swift helper beside the app.
func newMicrophone() platform.Microphone {
	return macMicrophone{helper: helper{}, binary: macosHelperPath()}
}
