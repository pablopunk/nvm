package real

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// pasteGesture sends Ctrl+V to the focused X11 window with xdotool. Ctrl+V is the 1.x gesture; terminals that paste
// with Ctrl+Shift+V are not covered.
func pasteGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keyGesture(ctx, helper, expectedAppID, "ctrl+v")
}

// undoGesture sends Ctrl+Z to the focused X11 window with xdotool.
func undoGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	return keyGesture(ctx, helper, expectedAppID, "ctrl+z")
}

// keyGesture sends one key chord to the focused X11 window with xdotool, after checking the focused window's class
// when expectedAppID is set.
func keyGesture(ctx context.Context, helper platform.Helper, expectedAppID, chord string) error {
	if !pasteCapable("linux", os.Getenv, programInstalled) {
		return platform.ErrUnsupported
	}
	if expectedAppID != "" {
		focused, err := runXdotool(ctx, helper, "getactivewindow", "getwindowclassname")
		if err != nil {
			return err
		}
		if !strings.EqualFold(focused, expectedAppID) {
			return platform.ErrConflict
		}
	}
	_, err := runXdotool(ctx, helper, "key", "--clearmodifiers", chord)
	return err
}

// runXdotool runs xdotool with args and returns its trimmed standard output. A missing binary is
// platform.ErrUnsupported; a non-zero exit is an error that names the failing command.
func runXdotool(ctx context.Context, helper platform.Helper, args ...string) (string, error) {
	result, err := helper.Run(ctx, xdotoolBinary, args, nil)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", platform.ErrUnsupported
		}
		return "", fmt.Errorf("xdotool %s: %w", args[0], err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("xdotool %s exited with %d", args[0], result.ExitCode)
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// frontmostAppID names the focused X11 window's class, the identity keyGesture checks before it types.
func frontmostAppID(ctx context.Context, helper platform.Helper, _ int) (string, error) {
	if !pasteCapable("linux", os.Getenv, programInstalled) {
		return "", platform.ErrUnsupported
	}
	return runXdotool(ctx, helper, "getactivewindow", "getwindowclassname")
}
