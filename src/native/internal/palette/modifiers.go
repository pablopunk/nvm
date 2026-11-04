package palette

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Mod is the shortcut modifier of the palette: ⌘ on macOS, Ctrl elsewhere.
var Mod = ui.Ctrl

func init() {
	if runtime.GOOS == "darwin" {
		Mod = ui.Cmd
	}
}

// KeyLabel spells a label written with ⌃ for the platform: the Control symbol becomes ⌘ on macOS.
func KeyLabel(label string) string {
	if runtime.GOOS == "darwin" {
		return strings.ReplaceAll(label, "⌃", "⌘")
	}
	return label
}
