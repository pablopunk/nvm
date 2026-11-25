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
	case protocol.OpPublishDisplay:
		return nil, h.publishDisplay(call.Args)
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

// settingSet stores a value after the capability check; saveSetting checks the value. It is saved before it returns, so
// the caller sees the value persisted.
func (h *Handler) settingSet(args json.RawMessage) (json.RawMessage, error) {
	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()
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
	return request.Value, h.saveSetting(definition, request.Value)
}

// settingToggle flips a boolean setting and returns the new value.
func (h *Handler) settingToggle(args json.RawMessage) (json.RawMessage, error) {
	h.settingsMu.Lock()
	defer h.settingsMu.Unlock()
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
	return next, h.saveSetting(definition, next)
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

// saveSetting checks a value against its definition and stores it where the extension's settings live: in the host's
// store for a built-in that has one, else in the extension's settings file.
func (h *Handler) saveSetting(definition api.SettingDefinition, value json.RawMessage) error {
	if err := checkSettingValue(definition, value); err != nil {
		return err
	}
	id := definition.ID
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
	if err := docs.settings.Commit(func(doc *settingValuesDoc) error {
		doc.Values[id] = value
		return nil
	}); err != nil {
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

// checkSettingValue refuses a value the definition does not accept, as api.SettingDefinition.CheckValue says. Every
// write passes it, whether the extension's settings file or the host's store keeps the setting.
func checkSettingValue(definition api.SettingDefinition, value json.RawMessage) error {
	if err := definition.CheckValue(value); err != nil {
		return invalid("%v", err)
	}
	return nil
}

func notFound(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, api.ErrNotFound)...)
}
