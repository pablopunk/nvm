package real

import (
	"context"
	"fmt"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// UNTESTED: this path has never run on macOS. It sends Command+V through System Events with osascript,
// which needs the Accessibility permission for the process that runs it (the terminal or the app's own
// bundle). The repo's Swift helper has no paste command, so a native CGEvent paste would be new Swift code;
// osascript is the smallest step that can be verified on a Mac runner (L5). Failures map to ErrDenied, the
// closest port error, because System Events reports a missing permission as a script error.
const macPasteScript = `tell application "System Events" to keystroke "v" using command down`

// pasteGesture runs the osascript paste. When expectedAppID is set, the paste is sent only while that bundle is in front.
func pasteGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	if expectedAppID != "" {
		current, err := frontmostAppID(ctx, helper, os.Getpid())
		if err != nil {
			return err
		}
		if current != expectedAppID {
			return platform.ErrConflict
		}
	}
	result, err := helper.Run(ctx, "osascript", []string{"-e", macPasteScript}, nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("paste keystroke refused (Accessibility permission?): %w", platform.ErrDenied)
	}
	return nil
}

// frontmostAppID names the bundle in front through Launch Services, or "" when Nevermind itself has the keyboard.
func frontmostAppID(ctx context.Context, helper platform.Helper, ownPID int) (string, error) {
	app, found, err := frontmostApp(ctx, helper)
	return frontmostIdentityOf(app, found, ownPID), err
}
