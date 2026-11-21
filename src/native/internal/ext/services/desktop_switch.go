package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

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
	if args.Direction != "next" && args.Direction != "previous" {
		return api.FocusResult{Reason: "invalid-spec", Message: "Desktop direction must be next or previous."}, nil
	}
	if h.deps.Desktops == nil || !h.deps.Desktops.Supported() {
		return api.FocusResult{Reason: "unavailable", Message: "Switching desktops is not available on this system."}, nil
	}
	if !h.actions.allow() {
		return api.FocusResult{}, rateExceeded("window moves")
	}
	result, err := h.runDesktopSwitch(ctx, args.Direction)
	if err != nil {
		return api.FocusResult{}, err
	}
	if result.ExitCode != 0 {
		h.reportAppleScriptFailure(result)
		return api.FocusResult{Reason: "failed", Message: desktopSwitchFailure}, nil
	}
	return api.FocusResult{OK: true}, nil
}

// runDesktopSwitch preserves the service's process-result contract while the native port owns the effect.
func (h *Handler) runDesktopSwitch(ctx context.Context, direction string) (api.ShellResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, defaultProcessTimeout)
	defer cancel()
	if h.closed.Load() {
		return api.ShellResult{}, errHandlerClosed
	}
	result, err := h.deps.Desktops.Switch(runCtx, direction)
	stdout := limitOutput(result.Stdout, defaultOutputLimit, result.Truncated)
	stderr := limitOutput(result.Stderr, defaultOutputLimit, result.Truncated)
	switch {
	case ctx.Err() != nil:
		return api.ShellResult{}, ctx.Err()
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return api.ShellResult{Stdout: stdout, Stderr: stderr + fmt.Sprintf("\nKilled after %dms", defaultProcessTimeout.Milliseconds()), ExitCode: -1}, nil
	case err != nil:
		return api.ShellResult{Stderr: err.Error(), ExitCode: 1}, nil
	}
	return api.ShellResult{Stdout: stdout, Stderr: stderr, ExitCode: result.ExitCode}, nil
}
