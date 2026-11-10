package real

import (
	"context"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type desktopSwitcher struct {
	commands platform.Commands
	goos     string
}

// NewDesktopSwitcher binds discovery and execution without probing permissions or starting a process.
func NewDesktopSwitcher(goos string, commands platform.Commands) platform.DesktopSwitcher {
	return desktopSwitcher{commands: commands, goos: goos}
}

func (s desktopSwitcher) Supported() bool { return s.goos == "darwin" && s.commands != nil }

func (s desktopSwitcher) Switch(ctx context.Context, direction string) (platform.HelperResult, error) {
	if err := ctx.Err(); err != nil {
		return platform.HelperResult{}, err
	}
	if !s.Supported() {
		return platform.HelperResult{}, platform.ErrUnsupported
	}
	var keyCode int
	switch direction {
	case "next":
		keyCode = 124
	case "previous":
		keyCode = 123
	default:
		return platform.HelperResult{}, fmt.Errorf("desktop direction must be next or previous")
	}
	script := fmt.Sprintf(`tell application "System Events" to key code %d using control down`, keyCode)
	return s.commands.Run(ctx, platform.CommandSpec{Binary: osascriptBinary, Args: []string{"-e", script}})
}
