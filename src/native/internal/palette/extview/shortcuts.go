package extview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// shortcutKeys are the keys an extension shortcut may name. Letters and digits are spelled by palette.KeyName.
var shortcutKeys = append(append([]ui.Key{
	ui.KeyEnter, ui.KeyBackspace, ui.KeyDelete, ui.KeyTab, ui.KeySpace, ui.KeyUp, ui.KeyDown, ui.KeyLeft,
	ui.KeyRight, ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown,
	ui.KeyF1, ui.KeyF2, ui.KeyF3, ui.KeyF4, ui.KeyF5, ui.KeyF6, ui.KeyF7, ui.KeyF8, ui.KeyF9, ui.KeyF10, ui.KeyF11, ui.KeyF12,
}, letterKeyList()...), digitKeyList()...)

func letterKeyList() []ui.Key {
	return []ui.Key{ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG, ui.KeyH, ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM,
		ui.KeyN, ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU, ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ}
}

func digitKeyList() []ui.Key {
	return []ui.Key{ui.Key0, ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9}
}

// parseShortcut reads an accelerator such as "CommandOrControl+E" or "Command+Backspace". CommandOrControl is the
// palette's modifier (Cmd on macOS, Ctrl elsewhere). ok is false for an accelerator with no key or an unknown key.
func parseShortcut(accelerator string) (mods ui.Modifiers, key ui.Key, ok bool) {
	parts := strings.Split(accelerator, "+")
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "commandorcontrol", "cmdorctrl":
			mods |= palette.Mod
		case "command", "cmd", "meta", "super":
			mods |= ui.Cmd
		case "control", "ctrl":
			mods |= ui.Ctrl
		case "alt", "option":
			mods |= ui.Alt
		case "shift":
			mods |= ui.Shift
		default:
			return 0, 0, false
		}
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	for _, candidate := range shortcutKeys {
		if strings.EqualFold(palette.KeyName(candidate), name) {
			return mods, candidate, true
		}
	}
	return 0, 0, false
}

// hyperKey is the Hyper key the palette labels shortcuts with; nil Deps.HyperKey means none.
func (s *Session) hyperKey() string {
	if s.deps.HyperKey == nil {
		return ""
	}
	return s.deps.HyperKey()
}

// shortcutLabel spells an accelerator the way the palette shows keys: ⌘ or ⌃, ⇧, ⌥, then the key. A shortcut that
// starts with the Hyper key shows ✦ in place of those keys.
func shortcutLabel(accelerator, hyper string) string {
	if rest, ok := shortcuts.StripHyper(accelerator, hyper); ok {
		return shortcuts.HyperSymbol + paletteKeyLabel(rest)
	}
	return paletteKeyLabel(accelerator)
}

func paletteKeyLabel(accelerator string) string {
	mods, key, ok := parseShortcut(accelerator)
	if !ok {
		return ""
	}
	var label string
	if mods&palette.Mod != 0 {
		label += palette.KeyLabel("⌃")
	}
	if mods&ui.Cmd != 0 && palette.Mod != ui.Cmd {
		label += palette.KeyLabel("⌃")
	}
	if mods&ui.Ctrl != 0 && palette.Mod != ui.Ctrl {
		label += "⌃"
	}
	if mods&ui.Alt != 0 {
		label += "⌥"
	}
	if mods&ui.Shift != 0 {
		label += "⇧"
	}
	switch key {
	case ui.KeyBackspace:
		return label + "⌫"
	case ui.KeyEnter:
		return label + "↵"
	}
	return label + palette.KeyName(key)
}
