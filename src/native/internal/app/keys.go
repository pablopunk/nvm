package app

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The palette follows the system theme: colors come from the design tokens, which follow ui.Theme. This
// file holds the platform's shortcut modifier, the one piece of the old theme file that is not a color.

// mod is the shortcut modifier: ⌘ on macOS, Ctrl elsewhere.
var mod = ui.Ctrl

func init() {
	if runtime.GOOS == "darwin" {
		mod = ui.Cmd
	}
}

// kb spells a Ctrl-based label for the platform (⌘ on macOS).
func kb(label string) string {
	if runtime.GOOS == "darwin" {
		return strings.ReplaceAll(label, "⌃", "⌘")
	}
	return label
}
