package shortcuts

import "strings"

var macSymbols = map[string]string{
	Command:          "⌘",
	CommandOrControl: "⌘",
	Control:          "⌃",
	Alt:              "⌥",
	Shift:            "⇧",
	Enter:            "↵",
	"Backspace":      "⌫",
	Escape:           "Esc",
	"Tab":            "Tab",
}

// Label spells an accelerator for a platform. On macOS it uses symbols with no separator (⌘⌥K). Elsewhere it uses
// key names joined by "+": Command is the meta key (Super on Linux, Win on Windows) and CommandOrControl is Control.
func Label(accelerator, goos string) string {
	parts := Parts(accelerator)
	if goos == "darwin" {
		for i, part := range parts {
			if symbol, ok := macSymbols[part]; ok {
				parts[i] = symbol
			}
		}
		return strings.Join(parts, "")
	}
	for i, part := range parts {
		parts[i] = keyNameOn(part, goos)
	}
	return strings.Join(parts, "+")
}

func keyNameOn(part, goos string) string {
	switch part {
	case Command:
		return metaKeyName(goos)
	case CommandOrControl:
		return Control
	}
	return part
}

// metaKeyName names the Command key on a system that has a different key for it.
func metaKeyName(goos string) string {
	if goos == "windows" {
		return "Win"
	}
	return "Super"
}
