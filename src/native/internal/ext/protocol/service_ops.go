package protocol

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Operations of the host services. Each service has the operations listed here; anything else is an invalid
// input. Arguments and results are the JSON shapes below, shared by the child and the host.
const (
	OpGet         = "get"
	OpSet         = "set"
	OpDelete      = "delete"
	OpClear       = "clear"
	OpDefinitions = "definitions"
	OpToggle      = "toggle"
	OpLog         = "log"
	OpRecent      = "recent"
	OpTemplate    = "template"
	OpFetch       = "fetch"

	OpRefreshMenuBar = "refreshMenuBar"
)

// KeyArgs names one entry: a storage key, a secret name, or a setting id.
type KeyArgs struct {
	Key string `json:"key"`
}

// KeyValueArgs stores one JSON value under a key, secret name or setting id.
type KeyValueArgs struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// GetResult is the answer to a get. Found is false when the entry does not exist, and Value is then empty.
type GetResult struct {
	Found bool            `json:"found"`
	Value json.RawMessage `json:"value,omitempty"`
}

// LogArgs is one record the extension wrote. Data is the JSON the extension passed, if any.
type LogArgs struct {
	Level   api.LogLevel    `json:"level"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// TemplateArgs is one Text.Template call.
type TemplateArgs struct {
	Input   string              `json:"input"`
	Options api.TemplateOptions `json:"options"`
}
