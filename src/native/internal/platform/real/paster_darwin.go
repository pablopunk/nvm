package real

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// UNTESTED: this path has never run on macOS. It sends Command+V through System Events with osascript,
// which needs the Accessibility permission for the process that runs it (the terminal or the app's own
// bundle). The repo's Swift helper has no paste command, so a native CGEvent paste would be new Swift code;
// osascript is the smallest step that can be verified on a Mac runner (L5). Failures map to ErrDenied, the
// closest port error, because System Events reports a missing permission as a script error.
const macPasteScript = `tell application "System Events" to keystroke "v" using command down`

// macUndoScript is the undo keystroke of the frontmost app, sent the same way as the paste.
const macUndoScript = `tell application "System Events" to keystroke "z" using command down`

// macTypeLines types the text argument (argv 1) and waits the delay argument (argv 2, seconds) between characters.
// The text arrives as an argument, never as script source, so no character of it is ever read as AppleScript.
var macTypeLines = []string{
	"on run argv",
	"set typed to item 1 of argv",
	"set pause to (item 2 of argv) as number",
	`tell application "System Events"`,
	"if pause is 0 then",
	"keystroke typed",
	"else",
	"repeat with ch in characters of typed",
	"keystroke ch",
	"delay pause",
	"end repeat",
	"end if",
	"end tell",
	"end run",
}

// pasteGesture runs the osascript paste. When expectedAppID is set, the paste is sent only while that bundle is in front.
func pasteGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keystrokeGesture(ctx, helper, expectedAppID, scriptArgs(macPasteScript))
}

// undoGesture runs the osascript undo, under the same expectedAppID rule as the paste.
func undoGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keystrokeGesture(ctx, helper, expectedAppID, scriptArgs(macUndoScript))
}

// typeGesture types text into the frontmost app, under the same expectedAppID rule as the paste.
func typeGesture(ctx context.Context, helper platform.Helper, expectedAppID, text string, delay time.Duration) error {
	args := make([]string, 0, 2*len(macTypeLines)+2)
	for _, line := range macTypeLines {
		args = append(args, "-e", line)
	}
	args = append(args, text, strconv.FormatFloat(delay.Seconds(), 'f', 3, 64))
	return keystrokeGesture(ctx, helper, expectedAppID, args)
}

// scriptArgs is the osascript argument list for one -e script.
func scriptArgs(script string) []string {
	return []string{"-e", script}
}

// keystrokeGesture runs osascript with args when expectedAppID is the bundle in front, or when it is "".
func keystrokeGesture(ctx context.Context, helper platform.Helper, expectedAppID string, args []string) error {
	if expectedAppID != "" {
		current, err := frontmostAppID(ctx, helper, os.Getpid())
		if err != nil {
			return err
		}
		if current != expectedAppID {
			return platform.ErrConflict
		}
	}
	result, err := helper.Run(ctx, "osascript", args, nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("keystroke refused (Accessibility permission?): %w", platform.ErrDenied)
	}
	return nil
}

// frontmostAppID names the bundle in front through Launch Services, or "" when Nevermind itself has the keyboard.
func frontmostAppID(ctx context.Context, helper platform.Helper, ownPID int) (string, error) {
	app, found, err := frontmostApp(ctx, helper)
	return frontmostIdentityOf(app, found, ownPID), err
}
