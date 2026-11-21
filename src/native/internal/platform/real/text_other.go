//go:build !darwin && !linux

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

func newScreenCapturer(platform.Helper) platform.ScreenCapturer {
	return unsupportedScreenCapturer{}
}

type unsupportedScreenCapturer struct{}

// Capture reports ErrUnsupported.
func (unsupportedScreenCapturer) Capture(context.Context, string, *platform.Rect) error {
	return platform.ErrUnsupported
}
