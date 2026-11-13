package api

import "fmt"

// SettingRef names one setting of one built-in: the setting whose published display text another built-in shows.
type SettingRef struct {
	Extension string `json:"extension"`
	Setting   string `json:"setting"`
}

// validateSharedDisplays checks the settings a built-in publishes the display of, which must be declared, and the
// shared settings it reads.
func (m Manifest) validateSharedDisplays() error {
	declared := map[string]bool{}
	for _, definition := range m.Settings {
		declared[definition.ID] = true
	}
	for _, id := range m.Publishes {
		if !declared[id] {
			return fmt.Errorf("extension %q publishes the display of %q, which it does not declare", m.ID, id)
		}
	}
	for _, ref := range m.Reads {
		if err := ref.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r SettingRef) validate() error {
	if !extensionIDPattern.MatchString(r.Extension) || !settingIDPattern.MatchString(r.Setting) {
		return fmt.Errorf("shared setting %q of %q is not a valid reference", r.Setting, r.Extension)
	}
	return nil
}

// validateSharedDisplayRow checks the shared setting a settings row shows, when it shows one.
func validateSharedDisplayRow(row SettingRow) error {
	if row.Shows == nil {
		return nil
	}
	if err := row.Shows.validate(); err != nil {
		return fmt.Errorf("settings row %q: %w", row.ID, err)
	}
	return nil
}
