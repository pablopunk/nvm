//go:build !linux && !darwin

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// pasteSupported is false on Windows: the SendInput paste needs a helper that this build does not ship, so
// the paste action is omitted rather than shown broken.
func pasteSupported() bool {
	return false
}

// pasteGesture reports that paste is unavailable on this OS.
func pasteGesture(context.Context, platform.Helper, string) error {
	return platform.ErrUnsupported
}
