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

// xdotoolBinary is the input tool the X11 paste uses. Wayland sessions have no equivalent here.
const xdotoolBinary = "xdotool"

// pasteSupported reports whether the session can paste: an X11 display, xdotool on the PATH, and no Wayland
// session (X11 apps under Xwayland receive no keys from xdotool on Wayland).
func pasteSupported() bool {
	if isWaylandSession() || os.Getenv("DISPLAY") == "" {
		return false
	}
	_, err := exec.LookPath(xdotoolBinary)
	return err == nil
}

// isWaylandSession reports whether the desktop session runs on Wayland.
func isWaylandSession() bool {
	return os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// pasteGesture sends Ctrl+V to the focused X11 window with xdotool, after checking the focused window's class
// when expectedAppID is set. Ctrl+V is the 1.x gesture; terminals that paste with Ctrl+Shift+V are not covered.
func pasteGesture(ctx context.Context, helper platform.Helper, expectedAppID string) error {
	if !pasteSupported() {
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
	_, err := runXdotool(ctx, helper, "key", "--clearmodifiers", "ctrl+v")
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
