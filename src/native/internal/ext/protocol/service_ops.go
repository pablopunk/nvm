package protocol

import (
	"encoding/json"
	"time"

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
	OpSave        = "save"
	OpCommit      = "commit"
	OpDiscard     = "discard"

	OpRecords = "records"

	OpRefreshMenuBar = "refreshMenuBar"
	OpInvalidateRoot = "invalidateRoot"

	OpReadImage  = "read-image"
	OpWriteImage = "write-image"

	OpShow          = "show"
	OpUpdate        = "update"
	OpHide          = "hide"
	OpRefreshSearch = "refreshSearch"
	OpOnce          = "once"
	OpCancel        = "cancel"
	OpPending       = "pending"
)

// OnceArgs schedules one run of the handler Ref names, at At, under Key.
type OnceArgs struct {
	Key string        `json:"key"`
	At  time.Time     `json:"at"`
	Ref api.ActionRef `json:"ref"`
}

// ScheduleKeyArgs names the scheduled run to cancel or ask about.
type ScheduleKeyArgs struct {
	Key string `json:"key"`
}

// PendingReply says whether a run is pending under the key, and when it runs.
type PendingReply struct {
	At      time.Time `json:"at"`
	Pending bool      `json:"pending"`
}

// IndicatorHideArgs names the indicator to hide. An empty ID is the extension's default indicator.
type IndicatorHideArgs struct {
	ID string `json:"id,omitempty"`
}

// BundleArgs names one bundled dataset the app ships.
type BundleArgs struct {
	Name string `json:"name"`
}

// KeyArgs names one entry: a storage key, a secret name, or a setting id.
type KeyArgs struct {
	Key string `json:"key"`
}

// SearchRefreshArgs names the search query a refresh is for.
type SearchRefreshArgs struct {
	Query string `json:"query"`
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

// DraftSaveArgs stores the unsaved text of one editor draft, typed on top of the version the view opened.
type DraftSaveArgs struct {
	Key     string `json:"key"`
	Version string `json:"version"`
	Content string `json:"content"`
}

// DraftCommitArgs names the draft the extension has persisted at Version.
type DraftCommitArgs struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

// DraftRecord is one stored draft. SavedAtMs is the host clock when the draft was last written.
type DraftRecord struct {
	Key       string `json:"key"`
	Version   string `json:"version"`
	Content   string `json:"content"`
	SavedAtMs int64  `json:"savedAtMs"`
}

// DraftLookup answers a drafts get. Draft is set only when Found is true.
type DraftLookup struct {
	Found bool        `json:"found"`
	Draft DraftRecord `json:"draft"`
}
