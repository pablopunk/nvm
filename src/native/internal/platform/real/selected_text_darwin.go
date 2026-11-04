package real

import (
	"context"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// UNTESTED: like the paste gesture, this has never run on macOS. It sends Command+C through System Events, which
// needs the Accessibility permission for the process that runs osascript. expectedAppID is not checked on macOS.
const macCopyScript = `tell application "System Events" to keystroke "c" using command down`

// copyGesture runs the osascript copy. A refused keystroke is reported as ErrDenied, as the paste gesture does.
func copyGesture(ctx context.Context, helper platform.Helper, _ string) error {
	result, err := helper.Run(ctx, "osascript", []string{"-e", macCopyScript}, nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("copy keystroke refused (Accessibility permission?): %w", platform.ErrDenied)
	}
	return nil
}
