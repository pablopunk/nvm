//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newCameras returns the macOS camera list, which the Swift helper beside the app reads from AVFoundation.
func newCameras() platform.Cameras {
	return macCameras{helper: helper{}, binary: macosHelperPath()}
}
