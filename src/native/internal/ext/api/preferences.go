package api

import "encoding/json"

// PreferenceRecord is one declared setting of an installed extension, as its settings page shows it to the user: the
// extension, the setting, its current value and the values it accepts (see SettingDefinition).
type PreferenceRecord struct {
	ExtensionID string          `json:"extensionId"`
	Extension   string          `json:"extension"`
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	Value       json.RawMessage `json:"value"`
	Choices     []SettingChoice `json:"choices,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
}

// Preferences reads and changes the declared settings of every installed extension, as the user does on their settings
// pages. List needs preferences; Set needs preferences.write and runs only from a palette action. Secrets are not
// preferences.
type Preferences interface {
	// List returns the settings whose extension or setting names hold every word of query; "" lists all.
	List(query string) []PreferenceRecord
	// Set saves a value the setting accepts and returns the setting as saved.
	Set(extensionID, settingID string, value any) (PreferenceRecord, error)
}
