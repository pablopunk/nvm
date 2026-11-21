//go:build !linux

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// List is unsupported on this OS until a macOS or Windows implementation exists; the capability is
// omitted from Capabilities, so features do not reach this call.
func (processes) List() ([]platform.Process, error) {
	return nil, platform.ErrUnsupported
}

// Signal is unsupported on this OS until a macOS or Windows implementation exists.
func (processes) Signal(pid int, sig platform.Signal) error {
	return platform.ErrUnsupported
}
