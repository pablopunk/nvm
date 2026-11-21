package real

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// The exit codes of the Swift helper's read and copy operations, the contract macos-selected-text.ts reads as well.
const (
	helperExitDenied        = 2
	helperExitNoSelection   = 3
	helperExitNotFrontmost  = 4
	helperExitModifiersHeld = 5
)

// macSelectionTarget is the pid of the app in front, the process both operations act on. Nevermind itself is never a
// target: the keyboard would be the palette's.
func macSelectionTarget(ctx context.Context, helper platform.Helper) (int, error) {
	app, found, err := frontmostApp(ctx, helper)
	if err != nil {
		return 0, err
	}
	if !found || app.PID == os.Getpid() {
		return 0, fmt.Errorf("no app other than Nevermind is in front: %w", platform.ErrConflict)
	}
	return app.PID, nil
}

// macSelectionCall runs one helper operation on the app in front and maps its exit code to a port error.
func macSelectionCall(ctx context.Context, helper platform.Helper, binary, operation string) ([]byte, error) {
	pid, err := macSelectionTarget(ctx, helper)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, macosCallTimeout)
	defer cancel()
	result, err := helper.Run(callCtx, binary, []string{operation, strconv.Itoa(pid)}, nil)
	if err != nil {
		return nil, err
	}
	switch result.ExitCode {
	case 0:
		return result.Stdout, nil
	case helperExitNoSelection:
		return nil, nil
	case helperExitDenied:
		return nil, fmt.Errorf("selected text %s refused (Accessibility permission?): %w", operation, platform.ErrDenied)
	case helperExitNotFrontmost:
		return nil, fmt.Errorf("the source app lost focus: %w", platform.ErrConflict)
	case helperExitModifiersHeld:
		return nil, fmt.Errorf("the shortcut's modifier keys were still held: %w", platform.ErrConflict)
	}
	return nil, fmt.Errorf("selected-text %s failed with status %d", operation, result.ExitCode)
}

// macAccessibleSelection reads the selection of the app in front through Accessibility. "" means there was none to read.
func macAccessibleSelection(ctx context.Context, helper platform.Helper, binary string) (string, error) {
	out, err := macSelectionCall(ctx, helper, binary, "read")
	return string(out), err
}

// macCopySelection sends Command+C to the app in front through the helper. The helper waits for the shortcut's own
// modifier keys to be released and posts the copy with exactly the Command flag, so Command+Control+Alt+R held down
// never turns the copy into another shortcut.
func macCopySelection(ctx context.Context, helper platform.Helper, binary string) error {
	_, err := macSelectionCall(ctx, helper, binary, "copy")
	return err
}
