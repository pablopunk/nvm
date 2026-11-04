package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

const toggleScript = `tell application "System Events" to tell appearance preferences to set dark mode to not dark mode`

// Commands lists the one command: a theme toggle, converted from a 1.x source file.
func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "toggle-theme", Title: "Toggle System Theme", Subtitle: "Switch between light and dark mode"}}, nil
}

// RunAction runs the AppleScript that flips dark mode. Like the 1.x source it reads no exit code, so a refused
// Automation grant reaches the user only through the host's own report.
func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "toggle-theme" {
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	_, err := ctx.Desktop().Shell().AppleScript(toggleScript, api.ShellOptions{})
	return api.Result{}, err
}
