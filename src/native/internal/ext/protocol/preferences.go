package protocol

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// PreferenceArgs names a declared setting of an extension and the value a set saves. Expect, when set, is the revision
// of the current value the change was prepared on: the host refuses the change when the value moved since.
type PreferenceArgs struct {
	ExtensionID string          `json:"extensionId"`
	SettingID   string          `json:"settingId"`
	Value       json.RawMessage `json:"value"`
	Expect      string          `json:"expect,omitempty"`
}

// PreferenceChange is a setting change the host prepared before anyone approves it: the setting as it is now, the
// value it will hold, and the revision of the current value. Host-only; extensions never read it.
type PreferenceChange struct {
	Current  api.PreferenceRecord
	Value    json.RawMessage
	Revision string
}
