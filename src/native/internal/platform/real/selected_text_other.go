//go:build !linux && !darwin

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// copyGesture reports that the copy gesture is unavailable on this OS; the capability is omitted there.
func copyGesture(context.Context, platform.Helper, string) error {
	return platform.ErrUnsupported
}

// accessibleSelection reports that no accessibility read exists on this OS.
func accessibleSelection(context.Context, platform.Helper) (string, error) {
	return "", platform.ErrUnsupported
}
