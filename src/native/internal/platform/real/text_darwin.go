//go:build darwin

package real

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// screencapture is the system screenshot tool; -x keeps it silent.
const screencapture = "/usr/sbin/screencapture"

// screenCaptureMac saves the screen with the system tool. A region is passed with -R when it has a size.
type screenCaptureMac struct {
	helper platform.Helper
}

func newScreenCapturer(helper platform.Helper) platform.ScreenCapturer {
	return screenCaptureMac{helper: helper}
}

// Capture writes a PNG to dst. The OS asks for screen recording permission on first use; a refusal is a failed run.
func (s screenCaptureMac) Capture(ctx context.Context, dst string, region *platform.Rect) error {
	args := []string{"-x"}
	if region != nil && region.Width > 0 && region.Height > 0 {
		args = append(args, "-R", strings.Join([]string{strconv.Itoa(region.X), strconv.Itoa(region.Y), strconv.Itoa(region.Width), strconv.Itoa(region.Height)}, ","))
	}
	args = append(args, dst)
	result, err := s.helper.Run(ctx, screencapture, args, nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		if strings.Contains(string(result.Stderr), "could not create image from display") {
			return fmt.Errorf("screen recording is not allowed: %w", platform.ErrDenied)
		}
		return errors.New(recognitionFailure(result.Stderr))
	}
	return nil
}
