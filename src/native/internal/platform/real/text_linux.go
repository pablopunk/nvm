//go:build linux

package real

import (
	"context"
	"errors"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// linuxScreenCapture saves the screen with whichever capture program this desktop has.
type linuxScreenCapture struct {
	helper  platform.Helper
	getenv  func(string) string
	hasTool func(string) bool
}

func newScreenCapturer(helper platform.Helper) platform.ScreenCapturer {
	return linuxScreenCapture{helper: helper, getenv: os.Getenv, hasTool: hasTool}
}

// Capture writes a PNG of the whole screen to dst. A region is refused: these programs disagree on how to take one, and
// on a scaled display cropping afterwards would read the wrong pixels.
func (s linuxScreenCapture) Capture(ctx context.Context, dst string, region *platform.Rect) error {
	if region != nil {
		return platform.ErrUnsupported
	}
	tool, ok := linuxScreenTool(s.getenv, s.hasTool)
	if !ok {
		return platform.ErrUnsupported
	}
	result, err := s.helper.Run(ctx, tool.binary, tool.args(dst), nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errors.New(recognitionFailure(result.Stderr))
	}
	return nil
}
