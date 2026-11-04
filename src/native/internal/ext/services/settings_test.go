package services

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func settingsManifest(id string, capabilities ...api.Capability) api.Manifest {
	manifest := manifestFor(id, capabilities...)
	manifest.Settings = []api.SettingDefinition{
		{ID: "units", Type: api.SettingString, Value: []byte(`"metric"`)},
		{ID: "compact", Type: api.SettingBoolean, Value: []byte(`false`)},
		{ID: "retries", Type: api.SettingNumber, Value: []byte(`3`)},
	}
	return manifest
}

func TestSettingsDefaultsReadBeforeAnyWrite(t *testing.T) {
	h := handlerFor(t, newRig(t), settingsManifest("acme.one"))
	got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "units"})))
	if !got.Found || string(got.Value) != `"metric"` {
		t.Fatalf("default = %+v; want the declared default", got)
	}
}

func TestSettingsWriteNeedsTheDeclaredCapability(t *testing.T) {
	h := handlerFor(t, newRig(t), settingsManifest("acme.one"))
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"imperial"`)}), api.CodePermission, "settings.write")
}

func TestSettingsWritesAreTypeCheckedAndSurviveReopen(t *testing.T) {
	r := newRig(t)
	h := handlerFor(t, r, settingsManifest("acme.one", api.CapabilitySettingsWrite))
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "retries", Value: []byte(`"three"`)}), api.CodeInvalid, "number")
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"imperial"`)}))
	if err := h.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	reopened := handlerFor(t, r, settingsManifest("acme.one", api.CapabilitySettingsWrite))
	got := decodeGet(t, mustOK(t, serve(t, reopened, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "units"})))
	if string(got.Value) != `"imperial"` {
		t.Fatalf("after reopen value = %s; want the saved value", got.Value)
	}
}

func TestUndeclaredSettingIsNotFound(t *testing.T) {
	h := handlerFor(t, newRig(t), settingsManifest("acme.one", api.CapabilitySettingsWrite))
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "secretFlag", Value: []byte(`true`)}), api.CodeNotFound, "secretFlag")
	if decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "secretFlag"}))).Found {
		t.Fatal("an undeclared setting was readable")
	}
}

func TestToggleFlipsBooleansOnly(t *testing.T) {
	h := handlerFor(t, newRig(t), settingsManifest("acme.one", api.CapabilitySettingsWrite))
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpToggle, protocol.KeyArgs{Key: "compact"}))
	got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "compact"})))
	if string(got.Value) != "true" {
		t.Fatalf("toggled value = %s; want true", got.Value)
	}
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpToggle, protocol.KeyArgs{Key: "units"}), api.CodeInvalid, "boolean")
}

func TestDefinitionsCarryCurrentValues(t *testing.T) {
	h := handlerFor(t, newRig(t), settingsManifest("acme.one", api.CapabilitySettingsWrite))
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "retries", Value: []byte(`5`)}))
	var definitions []api.SettingDefinition
	raw := mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpDefinitions, nil))
	if err := json.Unmarshal(raw, &definitions); err != nil {
		t.Fatalf("decode definitions: %v", err)
	}
	if len(definitions) != 3 || string(definitions[2].Value) != "5" {
		t.Fatalf("definitions = %+v; want three, with retries at 5", definitions)
	}
}
