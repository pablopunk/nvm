package real

import (
	"context"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// UNTESTED: this path has never run on macOS. It sends Command+V through System Events with osascript,
// which needs the Accessibility permission for the process that runs it (the terminal or the app's own
// bundle). The repo's Swift helper has no paste command, so a native CGEvent paste would be new Swift code;
// osascript is the smallest step that can be verified on a Mac runner (L5). Failures map to ErrDenied, the
// closest port error, because System Events reports a missing permission as a script error.
const macPasteScript = `tell application "System Events" to keystroke "v" using command down`

// pasteGesture runs the osascript paste. expectedAppID is not checked on macOS in this build.
func pasteGesture(ctx context.Context, helper platform.Helper, _ string) error {
	result, err := helper.Run(ctx, "osascript", []string{"-e", macPasteScript}, nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("paste keystroke refused (Accessibility permission?): %w", platform.ErrDenied)
	}
	return nil
}
