package services

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// memoryHostSettings is a host store that records what it was asked to save.
type memoryHostSettings struct {
	values map[string]json.RawMessage
	saves  []string
	fail   error
}

func (m *memoryHostSettings) Load(key string) (json.RawMessage, bool) {
	value, found := m.values[key]
	return value, found
}

func (m *memoryHostSettings) Save(key string, value json.RawMessage) error {
	if m.fail != nil {
		return m.fail
	}
	m.values[key] = value
	m.saves = append(m.saves, key)
	return nil
}

func builtInHandlerFor(t *testing.T, r rig, manifest api.Manifest, store SettingsHost) *Handler {
	t.Helper()
	ports := real.NewPorts()
	handler := NewBuiltIn(Deps{
		Files: ports.FS, Clock: r.clock, DataRoot: r.root, Keychain: r.keychain, Network: r.network, Clipboard: r.clipboard, Logs: r.logs,
	}, manifest, store)
	t.Cleanup(func() { _ = handler.Close() })
	return handler
}

// TestBuiltInSettingsLiveInTheHostStore checks that a built-in's reads, writes and toggles go to the host's store, and
// that a write is saved there before the call returns.
func TestBuiltInSettingsLiveInTheHostStore(t *testing.T) {
	r := newRig(t)
	store := &memoryHostSettings{values: map[string]json.RawMessage{"compact": json.RawMessage(`true`)}}
	h := builtInHandlerFor(t, r, settingsManifest("nevermind.settings", api.CapabilitySettingsWrite), store)

	if got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "compact"}))); string(got.Value) != "true" {
		t.Fatalf("stored value = %s; want the host's true", got.Value)
	}
	if got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "units"}))); string(got.Value) != `"metric"` {
		t.Fatalf("unstored value = %s; want the declared default", got.Value)
	}
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpToggle, protocol.KeyArgs{Key: "compact"}))
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"imperial"`)}))
	if string(store.values["compact"]) != "false" || string(store.values["units"]) != `"imperial"` {
		t.Fatalf("host store = %v; want the toggle and the write saved", store.values)
	}
	if len(store.saves) != 2 {
		t.Fatalf("saves = %v; want one per change", store.saves)
	}
}

func TestBuiltInSaveFailureIsReportedAsUnavailable(t *testing.T) {
	store := &memoryHostSettings{values: map[string]json.RawMessage{}, fail: errors.New("disk full")}
	h := builtInHandlerFor(t, newRig(t), settingsManifest("nevermind.settings", api.CapabilitySettingsWrite), store)
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"imperial"`)}), api.CodeUnavailable, "cannot be saved")
}
