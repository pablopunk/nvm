package rigtest

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform/folders"
)

// paletteHotkeyMissing reports whether the shared config.json under home names no palette shortcut. A fresh install
// binds none (the app shows "No shortcut set"), so a launch over such a HOME has no summon unless the rig writes one.
func paletteHotkeyMissing(home string) bool {
	data, err := os.ReadFile(filepath.Join(folders.Shared(home), folders.PortableConfigName))
	if err != nil {
		return true
	}
	var doc struct {
		Settings struct {
			PaletteHotkey string `json:"paletteHotkey"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return true
	}
	return doc.Settings.PaletteHotkey == ""
}
