package real

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// xdotoolTimeout bounds one call to the tool; a stuck X server must not hold the palette's hide.
const xdotoolTimeout = 2 * time.Second

// focusX11 reads and restores the focused window with xdotool, as the Electron app does on X11: the window is
// identified by its ID and its owning process, and restored only if that process still owns it.
type focusX11 struct{}

// newFocus returns the focus port of this OS.
func newFocus() platform.Focus { return focusX11{} }

// Active returns the focused window and the process that owns it.
func (focusX11) Active() (platform.FocusTarget, error) {
	id, err := xdotool("getactivewindow")
	if err != nil || !isWindowID(id) {
		return platform.FocusTarget{}, platform.ErrNotFound
	}
	pid, err := xdotool("getwindowpid", id)
	if err != nil {
		return platform.FocusTarget{}, platform.ErrNotFound
	}
	owner, err := strconv.Atoi(pid)
	if err != nil {
		return platform.FocusTarget{}, platform.ErrNotFound
	}
	return platform.FocusTarget{Window: id, PID: owner}, nil
}

// Restore activates the target window when its owner is still the process that had it focused.
func (focusX11) Restore(target platform.FocusTarget) error {
	pid, err := xdotool("getwindowpid", target.Window)
	if err != nil || pid != strconv.Itoa(target.PID) {
		return platform.ErrNotFound
	}
	_, err = xdotool("windowactivate", "--sync", target.Window)
	return err
}

// xdotool runs the tool on the session's display and returns its trimmed standard output.
func xdotool(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), xdotoolTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "xdotool", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// isWindowID reports whether s is the decimal window ID xdotool prints for a window.
func isWindowID(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}
