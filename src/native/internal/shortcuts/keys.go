package shortcuts

import "strings"

// Modifiers are the modifier keys held during a key press, as the recorder reads them.
type Modifiers struct {
	Command, Control, Alt, Shift bool
}

// Capture turns a key press into the accelerator the recorder stores, with the modifiers in canonical order before
// the key. key is the key's name, as the palette spells it (A, 5, Space, Up, F5). It returns "" for a press that is
// not a shortcut: no key, or a key without a modifier, unless the key is a function key or an arrow.
func Capture(mods Modifiers, key string) string {
	if key == "" {
		return ""
	}
	var parts []string
	for _, held := range []struct {
		down bool
		name string
	}{{mods.Command, Command}, {mods.Control, Control}, {mods.Alt, Alt}, {mods.Shift, Shift}} {
		if held.down {
			parts = append(parts, held.name)
		}
	}
	if len(parts) == 0 && !standsAlone(key) {
		return ""
	}
	return strings.Join(append(parts, key), "+")
}

var arrowKeys = map[string]bool{"Up": true, "Down": true, "Left": true, "Right": true}

func standsAlone(key string) bool {
	return arrowKeys[key] || isFunctionKey(key)
}

func isFunctionKey(key string) bool {
	if len(key) < 2 || key[0] != 'F' {
		return false
	}
	for _, r := range key[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
