package real

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	defaultsPath = "/usr/bin/defaults"
	plutilPath   = "/usr/bin/plutil"
)

// symbolicHotkeys reads the macOS shortcuts that are turned on from the user's com.apple.symbolichotkeys preferences,
// where System Settings keeps every entry with its enabled flag, key code and modifier mask.
type symbolicHotkeys struct{ helper platform.Helper }

// Enabled exports the preferences as an XML property list and converts it to JSON, which the standard library reads.
func (s symbolicHotkeys) Enabled(ctx context.Context) ([]platform.SystemShortcut, error) {
	exported, err := s.helper.Run(ctx, defaultsPath, []string{"export", "com.apple.symbolichotkeys", "-"}, nil)
	if err != nil || exported.ExitCode != 0 || exported.Truncated {
		return nil, fmt.Errorf("reading the symbolic hotkeys failed (exit %d): %v", exported.ExitCode, err)
	}
	converted, err := s.helper.Run(ctx, plutilPath, []string{"-convert", "json", "-o", "-", "-"}, exported.Stdout)
	if err != nil || converted.ExitCode != 0 || converted.Truncated {
		return nil, fmt.Errorf("converting the symbolic hotkeys failed (exit %d): %v", converted.ExitCode, err)
	}
	return parseSymbolicHotkeys(converted.Stdout)
}

type symbolicHotkeysFile struct {
	Entries map[string]struct {
		Enabled bool `json:"enabled"`
		Value   struct {
			Parameters []int64 `json:"parameters"`
		} `json:"value"`
	} `json:"AppleSymbolicHotKeys"`
}

// Modifier bits of the mask, as NSEvent defines them. Function and numeric-pad bits are ignored: arrow keys carry them
// without the user having pressed them.
const (
	maskShift   = 0x20000
	maskControl = 0x40000
	maskOption  = 0x80000
	maskCommand = 0x100000
)

// parseSymbolicHotkeys lists the enabled entries whose key this app can spell. An entry with no key (a modifier-only
// or unknown key code) cannot collide with a recorded shortcut, so it is left out.
func parseSymbolicHotkeys(data []byte) ([]platform.SystemShortcut, error) {
	var file symbolicHotkeysFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("reading the symbolic hotkeys failed: %w", err)
	}
	var out []platform.SystemShortcut
	for id, entry := range file.Entries {
		if !entry.Enabled || len(entry.Value.Parameters) < 3 {
			continue
		}
		key, ok := macKeyNames[int(entry.Value.Parameters[1])]
		if !ok {
			continue
		}
		mods := entry.Value.Parameters[2]
		out = append(out, platform.SystemShortcut{
			Name: symbolicHotkeyName(id), Key: key,
			Command: mods&maskCommand != 0, Control: mods&maskControl != 0,
			Alt: mods&maskOption != 0, Shift: mods&maskShift != 0,
		})
	}
	return out, nil
}

func symbolicHotkeyName(id string) string {
	if number, err := strconv.Atoi(id); err == nil {
		if name, ok := symbolicHotkeyNames[number]; ok {
			return name
		}
	}
	return "a macOS system shortcut"
}

// symbolicHotkeyNames names the entries of System Settings > Keyboard > Keyboard Shortcuts, by the IDs macOS stores them under.
var symbolicHotkeyNames = map[int]string{
	7: "Move focus to the menu bar", 8: "Move focus to the Dock", 9: "Move focus to the next window",
	10: "Move focus to the window toolbar", 11: "Move focus to the floating window",
	12: "Turn keyboard access on or off", 13: "Change the way Tab moves focus",
	15: "Turn zoom on or off", 17: "Zoom in", 19: "Zoom out", 21: "Invert colors",
	27: "Move focus to the next window of the app", 28: "Save picture of screen as a file",
	29: "Copy picture of screen to the clipboard", 30: "Save picture of selected area as a file",
	31: "Copy picture of selected area to the clipboard", 32: "Mission Control", 33: "Application windows",
	34: "Mission Control", 35: "Application windows", 36: "Show Desktop", 52: "Turn Dock hiding on or off",
	57: "Move focus to the status menus", 59: "Turn VoiceOver on or off", 60: "Select the previous input source",
	61: "Select the next input source", 64: "Spotlight search", 65: "Finder search window",
	79: "Move left a space", 80: "Move left a space", 81: "Move right a space", 82: "Move right a space",
	98: "Show the Help menu", 160: "Launchpad", 162: "Dock", 163: "Notification Center",
	175: "Do Not Disturb", 184: "Screenshot and recording options",
}

// macKeyNames spells the virtual key codes of a US keyboard as accelerators do.
var macKeyNames = map[int]string{
	0: "A", 1: "S", 2: "D", 3: "F", 4: "H", 5: "G", 6: "Z", 7: "X", 8: "C", 9: "V", 11: "B", 12: "Q", 13: "W", 14: "E",
	15: "R", 16: "Y", 17: "T", 18: "1", 19: "2", 20: "3", 21: "4", 22: "6", 23: "5", 24: "=", 25: "9", 26: "7",
	27: "-", 28: "8", 29: "0", 30: "]", 31: "O", 32: "U", 33: "[", 34: "I", 35: "P", 36: "Enter", 37: "L", 38: "J",
	39: "'", 40: "K", 41: ";", 42: "\\", 43: ",", 44: "/", 45: "N", 46: "M", 47: ".", 48: "Tab", 49: "Space",
	50: "`", 51: "Backspace", 53: "Escape", 96: "F5", 97: "F6", 98: "F7", 99: "F3", 100: "F8", 101: "F9", 103: "F11",
	109: "F10", 111: "F12", 118: "F4", 120: "F2", 122: "F1", 123: "Left", 124: "Right", 125: "Down", 126: "Up",
}
