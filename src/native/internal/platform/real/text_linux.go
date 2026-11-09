//go:build linux

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Linux has no screen capture in this build; the capability is absent and this is never reached through the ports.
func newScreenCapturer(platform.Helper) platform.ScreenCapturer {
	return unsupportedScreenCapturer{}
}

type unsupportedScreenCapturer struct{}

// Capture reports ErrUnsupported.
func (unsupportedScreenCapturer) Capture(context.Context, string, *platform.Rect) error {
	return platform.ErrUnsupported
}
