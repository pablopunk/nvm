package palette

import (
	"strings"
)

// shortcutSymbols spells the key names of a stored accelerator (Alt+Shift+Space) as the symbols the footers use.
var shortcutSymbols = map[string]string{
	"control":   "⌃",
	"ctrl":      "⌃",
	"alt":       "⌥",
	"option":    "⌥",
	"opt":       "⌥",
	"shift":     "⇧",
	"enter":     "↵",
	"return":    "↵",
	"backspace": "⌫",
	"escape":    "Esc",
	"esc":       "Esc",
}

// primaryModifierNames are the accelerator names of the primary modifier: ⌘ on macOS, ⌃ elsewhere, as KeyLabel
// spells the footers' ⌃.
var primaryModifierNames = map[string]bool{
	"command":          true,
	"cmd":              true,
	"commandorcontrol": true,
	"cmdorctrl":        true,
}

// ShortcutLabel spells a stored accelerator the way people read it: ⌥⇧Space rather than Alt+Shift+Space. Keys
// without a symbol keep their name, and a single letter is upper case.
func ShortcutLabel(accelerator string) string {
	var b strings.Builder
	for _, part := range strings.Split(accelerator, "+") {
		if part = strings.TrimSpace(part); part != "" {
			b.WriteString(shortcutPart(part))
		}
	}
	return b.String()
}

// shortcutPart spells one name of an accelerator.
func shortcutPart(part string) string {
	name := strings.ToLower(part)
	if primaryModifierNames[name] {
		return KeyLabel("⌃")
	}
	if symbol, ok := shortcutSymbols[name]; ok {
		return symbol
	}
	if len(part) == 1 {
		return strings.ToUpper(part)
	}
	return part
}
