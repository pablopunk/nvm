//go:build !linux && !darwin

package real

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// pasteGesture reports that paste is unavailable on this OS.
func pasteGesture(context.Context, platform.Helper, string) error {
	return platform.ErrUnsupported
}

// undoGesture reports that undo is unavailable on this OS.
func undoGesture(context.Context, platform.Helper, string) error {
	return platform.ErrUnsupported
}

// typeGesture reports that typing is unavailable on this OS.
func typeGesture(context.Context, platform.Helper, string, string, time.Duration) error {
	return platform.ErrUnsupported
}

// frontmostAppID reports that the app in front cannot be named on this OS.
func frontmostAppID(context.Context, platform.Helper, int) (string, error) {
	return "", platform.ErrUnsupported
}
