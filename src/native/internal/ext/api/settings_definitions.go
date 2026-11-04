package api

import (
	"fmt"
	"regexp"
)

// SettingType values understood by the host. An empty type accepts any JSON value.
const (
	SettingBoolean = "boolean"
	SettingString  = "string"
	SettingNumber  = "number"
)

const maxSettingIDLength = 64

var settingIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)

// validateSettingDefinitions checks the declared settings: ids are unique and well formed, and the type is one
// the host knows (or empty).
func validateSettingDefinitions(definitions []SettingDefinition) error {
	seen := map[string]bool{}
	for _, definition := range definitions {
		if len(definition.ID) > maxSettingIDLength || !settingIDPattern.MatchString(definition.ID) {
			return fmt.Errorf("setting id %q must start with a letter and use letters, digits, '_', '.' or '-' (at most %d characters)", definition.ID, maxSettingIDLength)
		}
		if seen[definition.ID] {
			return fmt.Errorf("setting id %q is declared twice", definition.ID)
		}
		seen[definition.ID] = true
		switch definition.Type {
		case "", SettingBoolean, SettingString, SettingNumber:
		default:
			return fmt.Errorf("setting %q has unknown type %q; use boolean, string or number", definition.ID, definition.Type)
		}
	}
	return nil
}
