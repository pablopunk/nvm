package windowmgmt

import "github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"

// SettingsDocument is the window management settings file the built-in reads once, as the values it starts from.
// Version 1 holds the geometry settings as they are.
type SettingsDocument struct {
	Settings geometry.Settings
}

// DefaultSettingsDocument is the document a fresh install starts from.
func DefaultSettingsDocument() SettingsDocument {
	return SettingsDocument{Settings: geometry.DefaultSettings()}
}
