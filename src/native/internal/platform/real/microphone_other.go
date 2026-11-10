//go:build !linux && !darwin

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// unsupportedMicrophone reports capture as unavailable until the OS has a capture path. Windows (WASAPI) is not built
// yet, so dictation says so instead of recording nothing; macOS records through the helper (microphone_darwin.go).
type unsupportedMicrophone struct{}

func newMicrophone() platform.Microphone {
	return unsupportedMicrophone{}
}

// Devices reports ErrUnsupported.
func (unsupportedMicrophone) Devices() ([]platform.MicrophoneDevice, error) {
	return nil, platform.ErrUnsupported
}

// Open reports ErrUnsupported.
func (unsupportedMicrophone) Open(context.Context, string, platform.AudioFormat) (platform.MicrophoneStream, error) {
	return nil, platform.ErrUnsupported
}
