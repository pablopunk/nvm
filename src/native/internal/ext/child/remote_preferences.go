package child

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remotePreferences reads and changes the declared settings of every extension through the host. A failed read reads
// as none.
type remotePreferences struct {
	link *hostLink
}

func (p remotePreferences) List(query string) []api.PreferenceRecord {
	var records []api.PreferenceRecord
	_ = p.link.Call(protocol.ServicePreferences, protocol.OpList, protocol.QueryArgs{Query: query}, &records)
	return records
}

// Set asks the host to save value as the setting, and returns the setting as saved.
func (p remotePreferences) Set(extensionID, settingID string, value any) (api.PreferenceRecord, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return api.PreferenceRecord{}, api.ErrInvalidInput
	}
	var saved api.PreferenceRecord
	args := protocol.PreferenceArgs{ExtensionID: extensionID, SettingID: settingID, Value: raw}
	err = p.link.Call(protocol.ServicePreferences, protocol.OpSet, args, &saved)
	return saved, err
}
