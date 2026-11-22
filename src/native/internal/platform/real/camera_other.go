//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// unsupportedCameras reports the list as unavailable. Camera views on these systems still list their cameras from the
// web view itself; only the host-owned, extension-visible list is missing.
type unsupportedCameras struct{}

func newCameras() platform.Cameras {
	return unsupportedCameras{}
}

// Devices reports ErrUnsupported.
func (unsupportedCameras) Devices() ([]platform.CameraDevice, error) {
	return nil, platform.ErrUnsupported
}
