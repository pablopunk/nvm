//go:build !linux && !darwin

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// pasteGesture reports that paste is unavailable on this OS.
func pasteGesture(context.Context, platform.Helper, string) error {
	return platform.ErrUnsupported
}
