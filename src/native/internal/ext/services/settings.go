package services

import (
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

const settingsFile = "settings.json"

// settingValuesDoc holds the values the user chose. A setting without a stored value reads as its default.
type settingValuesDoc struct {
	Values map[string]json.RawMessage `json:"values"`
}

func newSettingValues() settingValuesDoc {
	return settingValuesDoc{Values: map[string]json.RawMessage{}}
}

func (h *Handler) serveSettings(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpDefinitions:
		return h.settingDefinitions()
	case protocol.OpGet:
		return h.settingGet(call.Args)
	case protocol.OpSet:
		return h.settingSet(call.Args)
	case protocol.OpToggle:
		return h.settingToggle(call.Args)
	}
	return nil, invalid("settings has no operation %q", call.Op)
}

// settingDefinitions returns the declared settings with their current values.
func (h *Handler) settingDefinitions() ([]api.SettingDefinition, error) {
	definitions := make([]api.SettingDefinition, 0, len(h.manifest.Settings))
	for _, definition := range h.manifest.Settings {
		value, err := h.currentSetting(definition)
		if err != nil {
			return nil, err
		}
		definition.Value = value
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

func (h *Handler) settingGet(args json.RawMessage) (protocol.GetResult, error) {
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return protocol.GetResult{}, err
	}
	definition, found := h.declaredSetting(request.Key)
	if !found {
		return protocol.GetResult{}, nil
	}
	value, err := h.currentSetting(definition)
	if err != nil {
		return protocol.GetResult{}, err
	}
	return protocol.GetResult{Found: true, Value: value}, nil
}

// settingSet stores a value after the capability check and the type check. It is saved before it returns, so
// the caller sees the value persisted.
func (h *Handler) settingSet(args json.RawMessage) (json.RawMessage, error) {
	if err := h.require(api.CapabilitySettingsWrite); err != nil {
		return nil, err
	}
	var request protocol.KeyValueArgs
	if err := decodeArgs(args, &request); err != nil {
		return nil, err
	}
	definition, found := h.declaredSetting(request.Key)
	if !found {
		return nil, notFound("the extension declares no setting %q", request.Key)
	}
	if err := checkSettingType(definition, request.Value); err != nil {
		return nil, err
	}
	return request.Value, h.saveSetting(request.Key, request.Value)
}

// settingToggle flips a boolean setting and returns the new value.
func (h *Handler) settingToggle(args json.RawMessage) (json.RawMessage, error) {
	if err := h.require(api.CapabilitySettingsWrite); err != nil {
		return nil, err
	}
	var request protocol.KeyArgs
	if err := decodeArgs(args, &request); err != nil {
		return nil, err
	}
	definition, found := h.declaredSetting(request.Key)
	if !found {
		return nil, notFound("the extension declares no setting %q", request.Key)
	}
	if definition.Type != api.SettingBoolean {
		return nil, invalid("setting %q is not a boolean and cannot be toggled", request.Key)
	}
	value, err := h.currentSetting(definition)
	if err != nil {
		return nil, err
	}
	var current bool
	if err := json.Unmarshal(value, &current); err != nil {
		return nil, invalid("setting %q does not hold a boolean", request.Key)
	}
	next, _ := json.Marshal(!current)
	return next, h.saveSetting(request.Key, next)
}

// currentSetting is the stored value of a declared setting, or its default. A built-in whose settings the host keeps
// reads them from the host's store; every other extension reads its settings file.
func (h *Handler) currentSetting(definition api.SettingDefinition) (json.RawMessage, error) {
	if h.hostSettings != nil {
		if stored, found := h.hostSettings.Load(definition.ID); found {
			return stored, nil
		}
		return defaultValue(definition), nil
	}
	docs, err := h.openedDocuments()
	if err != nil {
		return nil, err
	}
	values, err := docs.settings.Load()
	if err != nil {
		return nil, unavailable("the extension's settings cannot be read: %v", err)
	}
	return currentValue(values, definition), nil
}

// saveSetting stores a value where the extension's settings live: in the host's store for a built-in that has one,
// else in the extension's settings file.
func (h *Handler) saveSetting(id string, value json.RawMessage) error {
	if h.hostSettings != nil {
		if err := h.hostSettings.Save(id, value); err != nil {
			return unavailable("the setting cannot be saved: %v", err)
		}
		return nil
	}
	docs, err := h.openedDocuments()
	if err != nil {
		return err
	}
	if err := docs.settings.Update(func(doc *settingValuesDoc) {
		doc.Values[id] = value
	}); err != nil {
		return unavailable("the setting cannot be saved: %v", err)
	}
	if err := docs.settings.Save(); err != nil {
		return unavailable("the setting cannot be saved: %v", err)
	}
	return nil
}

func (h *Handler) declaredSetting(id string) (api.SettingDefinition, bool) {
	for _, definition := range h.manifest.Settings {
		if definition.ID == id {
			return definition, true
		}
	}
	return api.SettingDefinition{}, false
}

// currentValue is the stored value of a setting, or its declared default when none was stored.
func currentValue(values settingValuesDoc, definition api.SettingDefinition) json.RawMessage {
	if stored, ok := values.Values[definition.ID]; ok {
		return stored
	}
	return defaultValue(definition)
}

// defaultValue is the declared default of a setting, or null when it declares none.
func defaultValue(definition api.SettingDefinition) json.RawMessage {
	if len(definition.Value) == 0 {
		return json.RawMessage("null")
	}
	return definition.Value
}

// checkSettingType refuses a value that does not match the declared type. An untyped setting accepts any JSON.
func checkSettingType(definition api.SettingDefinition, value json.RawMessage) error {
	if !json.Valid(value) {
		return invalid("the value of setting %q is not valid JSON", definition.ID)
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return invalid("the value of setting %q is not valid JSON", definition.ID)
	}
	ok := true
	switch definition.Type {
	case api.SettingBoolean:
		_, ok = decoded.(bool)
	case api.SettingString:
		_, ok = decoded.(string)
	case api.SettingNumber:
		_, ok = decoded.(float64)
	}
	if !ok {
		return invalid("setting %q expects a %s", definition.ID, definition.Type)
	}
	return nil
}

func notFound(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, api.ErrNotFound)...)
}
