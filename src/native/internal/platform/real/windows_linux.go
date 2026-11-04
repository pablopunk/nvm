package real

import (
	"context"
	"os"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// x11CallTimeout bounds one X11 tool call, so a stuck X server cannot hang a window command.
const x11CallTimeout = 5 * time.Second

// newWindowManager is nil unless the X11 window commands are capable here (see windowManagementCapable).
func newWindowManager(helper platform.Helper) platform.WindowManager {
	if !windowManagementCapable("linux", os.Getenv, programInstalled) {
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
