package settings

import "github.com/pablopunk/nvm/native/internal/ext/api"

// RecordedElsewhere names the settings this page records by pressing keys, which no typed value can set, with where to
// change each one instead. Their stored value is always empty: the shortcuts keep the real one.
func RecordedElsewhere() map[api.SettingRef]string {
	return map[api.SettingRef]string{
		{Extension: ID, Setting: SettingPaletteShortcut}: "it is a shortcut, so change it with Shortcuts().Set and the ActionID Shortcuts().Bindable() lists for it",
		{Extension: ID, Setting: SettingHyperKey}:        "the user records it by pressing the key on the Settings page; Shortcuts cannot set it",
	}
}
