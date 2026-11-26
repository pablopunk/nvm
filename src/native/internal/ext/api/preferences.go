package api

import "encoding/json"

// PreferenceRecord is one declared setting of an extension with its current value, as SettingDefinition describes it.
type PreferenceRecord struct {
	ExtensionID    string          `json:"extensionId"`
	ExtensionTitle string          `json:"extensionTitle"`
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Description    string          `json:"description,omitempty"`
	Type           string          `json:"type,omitempty"`
	Value          json.RawMessage `json:"value"`
	Choices        []SettingChoice `json:"choices,omitempty"`
	Min            *float64        `json:"min,omitempty"`
	Max            *float64        `json:"max,omitempty"`
}

// Preferences reads and changes every extension's declared settings, as the user does; never secrets. Set needs
// preferences.write and a palette action.
type Preferences interface {
	// List matches every word of query; "" lists all.
	List(query string) []PreferenceRecord
	// Set returns the setting as saved.
	Set(extensionID, settingID string, value any) (PreferenceRecord, error)
}
