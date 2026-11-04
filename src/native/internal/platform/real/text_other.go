//go:build !darwin && !linux

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Windows has no text recognizer or screen capture in this build; the capabilities are absent.
func newTextRecognizer(platform.Helper) platform.TextRecognizer {
	return unsupportedTextRecognizer{}
}

type unsupportedTextRecognizer struct{}

// Recognize reports ErrUnsupported.
func (unsupportedTextRecognizer) Recognize(context.Context, string, []string) (platform.TextResult, error) {
	return platform.TextResult{}, platform.ErrUnsupported
}

func newScreenCapturer(platform.Helper) platform.ScreenCapturer {
	return unsupportedScreenCapturer{}
}

type unsupportedScreenCapturer struct{}

// Capture reports ErrUnsupported.
func (unsupportedScreenCapturer) Capture(context.Context, string, *platform.Rect) error {
	return platform.ErrUnsupported
}
