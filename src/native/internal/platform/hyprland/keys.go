package hyprland

import (
	"fmt"
	"strings"
)

// combo is an accelerator in Hyprland's terms: modifier names and an xkb key name.
type combo struct {
	mods []string
	key  string
}

// The modifiers in the order Hyprland configs spell them. Command is the Super key on Linux, and CommandOrControl is
// Control there.
var modifierNames = map[string]string{
	"Command": "SUPER", "Super": "SUPER", "Meta": "SUPER",
	"Control": "CTRL", "CommandOrControl": "CTRL",
	"Alt":   "ALT",
	"Shift": "SHIFT",
}

var modifierOrder = []string{"SUPER", "CTRL", "ALT", "SHIFT"}

// keyNames maps canonical key names to xkb keysym names. Letters, digits and F1-F24 need no entry.
var keyNames = map[string]string{
	"Space": "space", "Enter": "Return", "Escape": "Escape", "Tab": "Tab", "Backspace": "BackSpace",
	"Delete": "Delete", "Insert": "Insert", "Home": "Home", "End": "End", "PageUp": "Prior", "PageDown": "Next",
	"Up": "Up", "Down": "Down", "Left": "Left", "Right": "Right",
	"Plus": "plus", "-": "minus", "=": "equal", ",": "comma", ".": "period", "/": "slash", ";": "semicolon",
	"'": "apostrophe", "`": "grave", "[": "bracketleft", "]": "bracketright", "\\": "backslash",
}

// parseAccelerator turns a canonical accelerator such as Control+Alt+K into a combo. A key Hyprland has no name for
// here, or an accelerator without a key, is an error, so nothing unbindable is written.
func parseAccelerator(accelerator string) (combo, error) {
	parts := strings.Split(accelerator, "+")
	if accelerator == "" || len(parts) == 0 {
		return combo{}, fmt.Errorf("no shortcut")
	}
	held := map[string]bool{}
	for _, part := range parts[:len(parts)-1] {
		name, ok := modifierNames[part]
		if !ok {
			return combo{}, fmt.Errorf("modifier %q has no Hyprland name", part)
		}
		held[name] = true
	}
	key, err := keyName(parts[len(parts)-1])
	if err != nil {
		return combo{}, err
	}
	var mods []string
	for _, name := range modifierOrder {
		if held[name] {
			mods = append(mods, name)
		}
	}
	return combo{mods: mods, key: key}, nil
}

func keyName(key string) (string, error) {
	if name, ok := keyNames[key]; ok {
		return name, nil
	}
	if len(key) == 1 && (key[0] >= 'A' && key[0] <= 'Z' || key[0] >= '0' && key[0] <= '9') {
		return key, nil
	}
	if isFunctionKey(key) {
		return key, nil
	}
	if _, isModifier := modifierNames[key]; isModifier {
		return "", fmt.Errorf("the shortcut has no key")
	}
	return "", fmt.Errorf("key %q has no Hyprland name", key)
}

func isFunctionKey(key string) bool {
	if len(key) < 2 || len(key) > 3 || key[0] != 'F' {
		return false
	}
	n := 0
	for _, r := range key[1:] {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 24
}

// modmask is the modifier mask "hyprctl binds -j" reports for the combo's modifiers.
func (c combo) modmask() int {
	bits := map[string]int{"SHIFT": 1, "CTRL": 4, "ALT": 8, "SUPER": 64}
	mask := 0
	for _, m := range c.mods {
		mask |= bits[m]
	}
	return mask
}
