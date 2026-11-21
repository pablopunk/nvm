package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SettingsHost keeps the settings of a built-in extension in the host's own store, such as config.json, so the values
// the user chose stay where the rest of the app keeps them. Load returns the stored value of a key and whether one is
// stored. Save stores a value and runs what the setting changes, and it returns before the call returns.
type SettingsHost interface {
	Load(key string) (json.RawMessage, bool)
	Save(key string, value json.RawMessage) error
}

// NewBuiltIn returns the services of a built-in extension whose settings the host keeps. Everything else is as New.
func NewBuiltIn(deps Deps, manifest api.Manifest, settings SettingsHost) *Handler {
	handler := New(deps, manifest)
	handler.hostSettings = settings
	return handler
}
