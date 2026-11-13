package shortcuts

import "strings"

// Modifiers are the modifier keys held during a key press, as the recorder reads them.
type Modifiers struct {
	Command, Control, Alt, Shift bool
}

// Or returns the modifiers held in either set.
func (m Modifiers) Or(other Modifiers) Modifiers {
	return Modifiers{
		Command: m.Command || other.Command,
		Control: m.Control || other.Control,
		Alt:     m.Alt || other.Alt,
		Shift:   m.Shift || other.Shift,
	}
}

// ModifiersAccelerator spells modifiers with no key, such as Command+Control+Alt, which is how the Hyper key is stored.
func ModifiersAccelerator(mods Modifiers) string {
	return strings.Join(ModifierNames(mods), "+")
}

// Capture turns a key press into the accelerator the recorder stores, with the modifiers in canonical order before
// the key. key is the key's name, as the palette spells it (A, 5, Space, Up, F5). It returns "" for a press that is
// not a shortcut: no key, or a key without a modifier, unless the key is a function key or an arrow.
func Capture(mods Modifiers, key string) string {
	if key == "" {
		return ""
	}
	parts := ModifierNames(mods)
	if len(parts) == 0 && !standsAlone(key) {
		return ""
	}
	return strings.Join(append(parts, key), "+")
}

// ModifierNames lists the held modifiers in canonical order, as accelerators spell them.
func ModifierNames(mods Modifiers) []string {
	var names []string
	for _, held := range []struct {
		down bool
		name string
	}{{mods.Command, Command}, {mods.Control, Control}, {mods.Alt, Alt}, {mods.Shift, Shift}} {
		if held.down {
			names = append(names, held.name)
		}
	}
	return names
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
