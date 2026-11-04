package real

import (
	"context"
	"os"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// x11CallTimeout bounds one X11 tool call, so a stuck X server cannot hang a window command.
const x11CallTimeout = 5 * time.Second

// windowManagementSupported reports whether this session has an X server to talk to. Wayland sessions
// without XWayland are omitted: X11 tools cannot move their windows.
func windowManagementSupported() bool {
	return os.Getenv("DISPLAY") != ""
}

func newWindowManager(helper platform.Helper) platform.WindowManager {
	if !windowManagementSupported() {
		return nil
	}
	return x11Windows{helper: helper}
}

// x11Windows drives windows through EWMH with the X11 tools a desktop ships: xprop reads properties,
// xwininfo reads geometry, xrandr lists monitors, and xdotool places and raises windows.
type x11Windows struct {
	helper platform.Helper
}

// call runs one X11 tool and returns its stdout. A non-zero exit is a failure with the tool's stderr.
func (x x11Windows) call(ctx context.Context, binary string, args ...string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, x11CallTimeout)
	defer cancel()
	result, err := x.helper.Run(callCtx, binary, args, nil)
	if err != nil {
		return "", &platform.WindowError{Code: platform.WindowErrorFailed, Message: err.Error()}
	}
	if result.ExitCode != 0 {
		return "", &platform.WindowError{Code: platform.WindowErrorFailed, Message: string(result.Stderr)}
	}
	return string(result.Stdout), nil
}
