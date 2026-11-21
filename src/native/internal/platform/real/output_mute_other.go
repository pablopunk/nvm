//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// unsupportedOutputMute reports that this OS cannot mute its output through the platform port.
type unsupportedOutputMute struct{}

func newOutputMute(platform.Helper) platform.OutputMuter {
	return unsupportedOutputMute{}
}

// Muted reports ErrUnsupported.
func (unsupportedOutputMute) Muted() (bool, error) {
	return false, platform.ErrUnsupported
}

// SetMuted reports ErrUnsupported.
func (unsupportedOutputMute) SetMuted(bool) error {
	return platform.ErrUnsupported
}
