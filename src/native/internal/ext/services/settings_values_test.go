package services

import (
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

func boundedSettingsManifest(id string) api.Manifest {
	low, high := 1.0, 25.0
	manifest := manifestFor(id, api.CapabilitySettingsWrite)
	manifest.Settings = []api.SettingDefinition{
		{ID: "units", Type: api.SettingString, Value: []byte(`"metric"`),
			Choices: []api.SettingChoice{{Value: "metric", Title: "Metric"}, {Value: "imperial", Title: "Imperial"}}},
		{ID: "step", Type: api.SettingNumber, Value: []byte(`5`), Min: &low, Max: &high},
	}
	return manifest
}

// TestWritesOutsideDeclaredValuesAreRefused checks that a write outside the declared choices or range is refused and
// leaves the stored value alone, for an extension's settings file.
func TestWritesOutsideDeclaredValuesAreRefused(t *testing.T) {
	h := handlerFor(t, newRig(t), boundedSettingsManifest("acme.bounded"))
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "step", Value: []byte(`40`)}), api.CodeInvalid, "from 1 to 25")
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"furlongs"`)}), api.CodeInvalid, "accepts only")
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "step", Value: []byte(`10`)}))
	if got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "step"}))); string(got.Value) != "10" {
		t.Fatalf("step = %s; want the accepted 10", got.Value)
	}
	if got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "units"}))); string(got.Value) != `"metric"` {
		t.Fatalf("units = %s; want the default after a refused write", got.Value)
	}
}

// TestHostStoreNeverSeesARefusedValue checks the same refusal for a built-in whose settings the host keeps.
func TestHostStoreNeverSeesARefusedValue(t *testing.T) {
	store := &memoryHostSettings{values: map[string]json.RawMessage{}}
	h := builtInHandlerFor(t, newRig(t), boundedSettingsManifest("nevermind.settings"), store)
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "step", Value: []byte(`0`)}), api.CodeInvalid, "from 1 to 25")
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"furlongs"`)}), api.CodeInvalid, "accepts only")
	if len(store.saves) != 0 {
		t.Fatalf("host store saves = %v; want none for refused values", store.saves)
	}
	mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"imperial"`)}))
	if string(store.values["units"]) != `"imperial"` {
		t.Fatalf("host store = %v; want the accepted choice saved", store.values)
	}
}

func TestDefinitionsCarryDeclaredValues(t *testing.T) {
	h := handlerFor(t, newRig(t), boundedSettingsManifest("acme.bounded"))
	var definitions []api.SettingDefinition
	if err := json.Unmarshal(mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpDefinitions, nil)), &definitions); err != nil {
		t.Fatalf("decode definitions: %v", err)
	}
	if len(definitions[0].Choices) != 2 || definitions[1].Min == nil || *definitions[1].Max != 25 {
		t.Fatalf("definitions = %+v; want the choices and the range", definitions)
	}
}
