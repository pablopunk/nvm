package palette

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Mod is the shortcut modifier of the palette: ⌘ on macOS, Ctrl elsewhere.
var Mod = ModOn(runtime.GOOS)

// ModOn is the shortcut modifier of the palette on goos, so a platform's binding and its labels can be checked together.
func ModOn(goos string) ui.Modifiers {
	if goos == "darwin" {
		return ui.Cmd
	}
	return ui.Ctrl
}

// KeyLabel spells a label written with ⌃ for the platform: the Control symbol becomes ⌘ on macOS.
func KeyLabel(label string) string {
	return KeyLabelOn(label, runtime.GOOS)
}

// KeyLabelOn spells a label written with ⌃ for goos: the Control symbol becomes ⌘ on darwin.
func KeyLabelOn(label, goos string) string {
	if goos == "darwin" {
		return strings.ReplaceAll(label, "⌃", "⌘")
	}
	return label
}
