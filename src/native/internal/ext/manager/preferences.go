package manager

import (
	"encoding/json"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ExtensionPreferences is one runnable extension's declared settings that the user can change, in declaration order.
type ExtensionPreferences struct {
	ID       string
	Title    string
	Settings []api.SettingDefinition
}

// DeclaredPreferences lists the settings of every runnable extension that declares any, sorted by folder. A setting the
// manifest lists in Publishes is display text for another built-in's row, not a value the user chooses, so it is left
// out.
func (m *Manager) DeclaredPreferences() []ExtensionPreferences {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ExtensionPreferences
	for _, key := range m.sortedKeysLocked() {
		e := m.entries[key]
		if e.host == nil || !m.isRunnable(e) {
			continue
		}
		manifest := e.bundle.Manifest
		settings := userChosenSettings(manifest)
		if len(settings) > 0 {
			out = append(out, ExtensionPreferences{ID: manifest.ID, Title: manifest.Title, Settings: settings})
		}
	}
	return out
}

func userChosenSettings(manifest api.Manifest) []api.SettingDefinition {
	var settings []api.SettingDefinition
	for _, definition := range manifest.Settings {
		if !slices.Contains(manifest.Publishes, definition.ID) {
			settings = append(settings, definition)
		}
	}
	return settings
}

// SaveSettingForUser stores value as the declared setting key of runnable extension id, as the user's own change on a
// settings page: the value is checked, but the extension needs no settings.write. It is saved before it returns.
func (m *Manager) SaveSettingForUser(id, key string, value json.RawMessage) error {
	m.mu.Lock()
	e, err := m.findLocked(id)
	if err == nil && (e.host == nil || !m.isRunnable(e)) {
		err = ErrUnknownExtension
	}
	m.mu.Unlock()
	if err != nil {
		return err
	}
	return e.host.SaveSettingForUser(key, value)
}
