package rigtest

import (
	"fmt"
	"strings"
)

// xdotoolModifiers maps accelerator modifiers, as the app stores them (Alt+Shift+Space), to xdotool.
var xdotoolModifiers = map[string]string{
	"alt": "alt", "option": "alt",
	"shift": "shift",
	"ctrl":  "ctrl", "control": "ctrl", "cmdorctrl": "ctrl",
	"cmd": "super", "command": "super", "super": "super", "meta": "super",
}

// xdotoolKeys maps named accelerator keys to xdotool keysym names; single characters pass through.
var xdotoolKeys = map[string]string{
	"space": "space", "esc": "Escape", "escape": "Escape", "enter": "Return", "return": "Return",
	"tab": "Tab", "backspace": "BackSpace", "delete": "Delete",
	"up": "Up", "down": "Down", "left": "Left", "right": "Right",
}

// XdotoolKeys converts an accelerator such as "Alt+Shift+Space" into an xdotool key chord
// ("alt+shift+space"). Unknown modifiers or keys are errors, so a typo fails the test instead of
// sending the wrong keys.
func XdotoolKeys(accelerator string) (string, error) {
	parts := strings.Split(accelerator, "+")
	if len(parts) == 0 || strings.TrimSpace(accelerator) == "" {
		return "", fmt.Errorf("empty accelerator")
	}
	keys := make([]string, 0, len(parts))
	for i, part := range parts {
		name := strings.TrimSpace(part)
		lower := strings.ToLower(name)
		if i < len(parts)-1 {
			mapped, ok := xdotoolModifiers[lower]
			if !ok {
				return "", fmt.Errorf("unknown modifier %q in %q", name, accelerator)
			}
			keys = append(keys, mapped)
			continue
		}
		key, err := xdotoolKeyName(name, lower)
		if err != nil {
			return "", fmt.Errorf("%w in %q", err, accelerator)
		}
		keys = append(keys, key)
	}
	return strings.Join(keys, "+"), nil
}

func xdotoolKeyName(name, lower string) (string, error) {
	if mapped, ok := xdotoolKeys[lower]; ok {
		return mapped, nil
	}
	if len(name) == 1 {
		return lower, nil
	}
	if len(lower) >= 2 && lower[0] == 'f' && isDigits(lower[1:]) {
		return "F" + lower[1:], nil
	}
	return "", fmt.Errorf("unknown key %q", name)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
