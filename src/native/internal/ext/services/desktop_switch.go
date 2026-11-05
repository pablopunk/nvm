package services

import (
	"context"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// desktopKeyCodes are the arrow keys of the Mission Control shortcuts that move one space right or left.
var desktopKeyCodes = map[string]int{"next": 124, "previous": 123}

// desktopSwitchFailure is what the user sees when macOS refuses the keys that switch desktops: the two Mission Control
// shortcuts must be on, and System Events must be allowed to receive the keys.
const desktopSwitchFailure = "Could not switch desktop. Turn on “Move left a space” and “Move right a space” in Settings › " +
	"Keyboard › Keyboard Shortcuts › Mission Control, and allow Nevermind to control System Events in Privacy & Security › " +
	"Automation."

// switchDesktop sends the Mission Control key that moves one space. The keys go to the app in front, as the shortcut
// does, so the palette is never needed. Failures come back as results with OK false.
func (h *Handler) switchDesktop(ctx context.Context, call protocol.ServiceCall) (api.FocusResult, error) {
	var args protocol.SwitchDesktopArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return api.FocusResult{}, err
	}
	keyCode, ok := desktopKeyCodes[args.Direction]
	if !ok {
		return api.FocusResult{Reason: "invalid-spec", Message: "Desktop direction must be next or previous."}, nil
	}
	if h.deps.OS != "darwin" {
		return api.FocusResult{Reason: "unavailable", Message: "Switching desktops is not available on this system."}, nil
	}
	if !h.actions.allow() {
		return api.FocusResult{}, rateExceeded("window moves")
	}
	script := fmt.Sprintf(`tell application "System Events" to key code %d using control down`, keyCode)
	result, err := h.run(ctx, platform.CommandSpec{Binary: osascriptBinary, Args: []string{"-e", script}}, api.ShellOptions{})
	if err != nil {
		return api.FocusResult{}, err
	}
	if result.ExitCode != 0 {
		h.reportAppleScriptFailure(result)
		return api.FocusResult{Reason: "failed", Message: desktopSwitchFailure}, nil
	}
	return api.FocusResult{OK: true}, nil
}
