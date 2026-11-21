package manager

import (
	"encoding/json"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// settingCallBudget bounds one host-side read or write of a setting. The settings file is local, so the bound is generous.
const settingCallBudget = 5 * time.Second

// ReadSetting decodes the value of the declared setting key of extension id into into. The host reads a setting the
// same way the extension's own Settings().Get does; an undeclared key is api.ErrNotFound.
func (m *Manager) ReadSetting(id, key string, into any) error {
	var result protocol.GetResult
	if err := m.callService(id, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: key}, &result, settingCallBudget); err != nil {
		return err
	}
	if !result.Found {
		return api.ErrNotFound
	}
	return json.Unmarshal(result.Value, into)
}

// WriteSetting stores value as the declared setting key of extension id, saved before it returns. The extension must
// declare the settings.write capability, which the host checks.
func (m *Manager) WriteSetting(id, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return api.ErrInvalidInput
	}
	return m.callService(id, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: key, Value: raw}, nil, settingCallBudget)
}

// SettingValue returns the stored value of the declared setting key of extension id, or its default when none is stored.
// It reads through the extension's settings service, as the extension's own Settings().Get does.
func (m *Manager) SettingValue(id, key string) (json.RawMessage, error) {
	var result protocol.GetResult
	if err := m.callService(id, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: key}, &result, settingCallBudget); err != nil {
		return nil, err
	}
	if !result.Found {
		return nil, api.ErrNotFound
	}
	return result.Value, nil
}
