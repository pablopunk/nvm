//go:build linux

package real

import (
	"context"
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// tesseractText reads text with Tesseract's command line, which the user installs from their package manager.
type tesseractText struct {
	helper platform.Helper
}

func newTextRecognizer(helper platform.Helper) platform.TextRecognizer {
	return tesseractText{helper: helper}
}

// Recognize runs `tesseract <path> stdout -l <languages> tsv` and parses the word rows of the output.
func (t tesseractText) Recognize(ctx context.Context, path string, languages []string) (platform.TextResult, error) {
	checked, err := checkLanguages(languages)
	if err != nil {
		return platform.TextResult{}, err
	}
	args := []string{path, "stdout"}
	if len(checked) > 0 {
		args = append(args, "-l", strings.Join(checked, "+"))
	}
	args = append(args, "tsv")
	result, err := t.helper.Run(ctx, tesseractBinary, args, nil)
	if err != nil {
		return platform.TextResult{}, err
	}
	if result.ExitCode != 0 {
		return platform.TextResult{}, errors.New(recognitionFailure(result.Stderr))
	}
	return parseTesseractTSV(result.Stdout, checked)
}

// Linux has no screen capture in this build; the capability is absent and this is never reached through the ports.
func newScreenCapturer(platform.Helper) platform.ScreenCapturer {
	return unsupportedScreenCapturer{}
}

type unsupportedScreenCapturer struct{}

// Capture reports ErrUnsupported.
func (unsupportedScreenCapturer) Capture(context.Context, string, *platform.Rect) error {
	return platform.ErrUnsupported
}
