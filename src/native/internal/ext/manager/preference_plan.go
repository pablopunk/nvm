package manager

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// planOn prepares saving value as the setting, against the source as it is now. The change carries the setting and the
// revision of its value whenever the setting exists, also when the change is refused.
func (e PreferenceEditor) planOn(source PreferenceSource, extensionID, settingID string, value json.RawMessage) (protocol.PreferenceChange, error) {
	extension, found := findExtensionPreferences(source, extensionID)
	if !found {
		return protocol.PreferenceChange{}, fmt.Errorf("no extension %q has preferences; list them first: %w", extensionID, api.ErrInvalidInput)
	}
	definition, found := findDefinition(extension, settingID)
	if !found {
		return protocol.PreferenceChange{}, fmt.Errorf("%s has no setting %q; list the preferences first: %w", extension.Title, settingID, api.ErrInvalidInput)
	}
	current, err := recordOf(source, extension, definition)
	if err != nil {
		return protocol.PreferenceChange{}, preferenceRefusal(err)
	}
	change := protocol.PreferenceChange{Current: current, Value: value, Revision: revisionOf(current.Value)}
	if where, recorded := e.recordedElsewhere[api.SettingRef{Extension: extensionID, Setting: settingID}]; recorded {
		return change, fmt.Errorf("“%s” is not changed here: %s: %w", definition.Title, where, api.ErrInvalidInput)
	}
	if err := definition.CheckValue(value); err != nil {
		return change, fmt.Errorf("%w: %w", err, api.ErrInvalidInput)
	}
	if sameValue(current.Value, value) {
		return change, fmt.Errorf("“%s” already has this value; nothing changed: %w", definition.Title, api.ErrInvalidInput)
	}
	return change, nil
}

// records are the settings the user can change here, with their values now. A setting whose value cannot be read is
// left out.
func (e PreferenceEditor) records(source PreferenceSource) []api.PreferenceRecord {
	var out []api.PreferenceRecord
	for _, extension := range source.DeclaredPreferences() {
		for _, definition := range extension.Settings {
			if _, recorded := e.recordedElsewhere[api.SettingRef{Extension: extension.ID, Setting: definition.ID}]; recorded {
				continue
			}
			if record, err := recordOf(source, extension, definition); err == nil {
				out = append(out, record)
			}
		}
	}
	return out
}

func recordOf(source PreferenceSource, extension ExtensionPreferences, definition api.SettingDefinition) (api.PreferenceRecord, error) {
	value, err := source.SettingValue(extension.ID, definition.ID)
	if err != nil {
		return api.PreferenceRecord{}, err
	}
	return api.PreferenceRecord{
		ExtensionID: extension.ID, Extension: extension.Title, ID: definition.ID, Title: definition.Title,
		Description: definition.Description, Type: definition.Type, Value: value, Choices: definition.Choices,
		Min: definition.Min, Max: definition.Max,
	}, nil
}

// matchingRecords keeps the records whose extension or setting names hold every word of query, ignoring case.
func matchingRecords(records []api.PreferenceRecord, query string) []api.PreferenceRecord {
	words := strings.Fields(strings.ToLower(query))
	var out []api.PreferenceRecord
	for _, record := range records {
		text := strings.ToLower(strings.Join([]string{record.ExtensionID, record.Extension, record.ID, record.Title, record.Description}, " "))
		if holdsEveryWord(text, words) {
			out = append(out, record)
		}
	}
	return out
}

func holdsEveryWord(text string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func findExtensionPreferences(source PreferenceSource, id string) (ExtensionPreferences, bool) {
	for _, extension := range source.DeclaredPreferences() {
		if extension.ID == id {
			return extension, true
		}
	}
	return ExtensionPreferences{}, false
}

func findDefinition(extension ExtensionPreferences, id string) (api.SettingDefinition, bool) {
	for _, definition := range extension.Settings {
		if definition.ID == id {
			return definition, true
		}
	}
	return api.SettingDefinition{}, false
}

// revisionOf names a setting's value: two values that decode to the same JSON have the same revision.
func revisionOf(value json.RawMessage) string {
	sum := sha256.Sum256([]byte(canonicalJSON(value)))
	return hex.EncodeToString(sum[:8])
}

func sameValue(a, b json.RawMessage) bool {
	return canonicalJSON(a) == canonicalJSON(b)
}

// canonicalJSON re-encodes a value so spacing and key order do not tell two equal values apart. Invalid JSON stays as
// it is.
func canonicalJSON(value json.RawMessage) string {
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return string(value)
	}
	encoded, _ := json.Marshal(decoded)
	return string(encoded)
}
