package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// recordingPreferences is a preference editor that lists one setting and records the changes it is asked for.
type recordingPreferences struct {
	queries []string
	saved   []protocol.PreferenceArgs
}

func (p *recordingPreferences) List(_ context.Context, query string) ([]api.PreferenceRecord, error) {
	p.queries = append(p.queries, query)
	return []api.PreferenceRecord{{ExtensionID: "acme.units", ID: "units", Title: "Units", Value: json.RawMessage(`"metric"`)}}, nil
}

func (p *recordingPreferences) Plan(context.Context, string, string, json.RawMessage) (protocol.PreferenceChange, error) {
	return protocol.PreferenceChange{}, nil
}

func (p *recordingPreferences) Set(_ context.Context, extensionID, settingID string, value json.RawMessage, expect string) (api.PreferenceRecord, error) {
	p.saved = append(p.saved, protocol.PreferenceArgs{ExtensionID: extensionID, SettingID: settingID, Value: value, Expect: expect})
	return api.PreferenceRecord{ExtensionID: extensionID, ID: settingID, Value: value}, nil
}

func servePreference(h *Handler, method, op string, args any) protocol.ServiceReply {
	raw, _ := json.Marshal(args)
	ctx := protocol.WithRequestMethod(context.Background(), method)
	return h.Serve(ctx, protocol.ServiceCall{CallID: 1, Service: protocol.ServicePreferences, Op: op, Args: raw})
}

var imperialUnits = protocol.PreferenceArgs{ExtensionID: "acme.units", SettingID: "units", Value: json.RawMessage(`"imperial"`)}

func TestListingPreferencesNeedsThePreferencesCapability(t *testing.T) {
	editor := &recordingPreferences{}
	undeclared := New(Deps{Preferences: editor}, manifestFor("acme.other"))
	requireCode(t, servePreference(undeclared, protocol.MethodSearch, protocol.OpList, protocol.QueryArgs{}), api.CodePermission, "preferences")
	reader := New(Deps{Preferences: editor}, manifestFor("acme.reader", api.CapabilityPreferences))
	var records []api.PreferenceRecord
	if err := json.Unmarshal(mustOK(t, servePreference(reader, protocol.MethodSearch, protocol.OpList, protocol.QueryArgs{Query: "unit"})), &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(editor.queries) != 1 || editor.queries[0] != "unit" {
		t.Fatalf("records = %+v, queries = %v", records, editor.queries)
	}
}

func TestChangingAPreferenceNeedsTheWriteCapabilityAndAPaletteRun(t *testing.T) {
	editor := &recordingPreferences{}
	reader := New(Deps{Preferences: editor}, manifestFor("acme.reader", api.CapabilityPreferences))
	requireCode(t, servePreference(reader, protocol.MethodRun, protocol.OpSet, imperialUnits), api.CodePermission, "preferences.write")
	writer := New(Deps{Preferences: editor}, manifestFor("acme.writer", api.CapabilityPreferences, api.CapabilityPreferencesWrite))
	requireCode(t, servePreference(writer, protocol.MethodSearch, protocol.OpSet, imperialUnits), api.CodePermission, "palette action")
	if len(editor.saved) != 0 {
		t.Fatalf("a refused change reached the editor: %+v", editor.saved)
	}
	mustOK(t, servePreference(writer, protocol.MethodRun, protocol.OpSet, imperialUnits))
	mustOK(t, servePreference(writer, protocol.MethodProgram, protocol.OpSet, imperialUnits))
	if len(editor.saved) != 2 || editor.saved[0].SettingID != "units" || string(editor.saved[0].Value) != `"imperial"` {
		t.Fatalf("saved = %+v", editor.saved)
	}
}

func TestAPreferenceChangeNeedsTheExtensionAndTheSetting(t *testing.T) {
	writer := New(Deps{Preferences: &recordingPreferences{}}, manifestFor("acme.writer", api.CapabilityPreferences, api.CapabilityPreferencesWrite))
	requireCode(t, servePreference(writer, protocol.MethodRun, protocol.OpSet, protocol.PreferenceArgs{SettingID: "units", Value: json.RawMessage(`1`)}), api.CodeInvalid, "extension id")
	requireCode(t, servePreference(writer, protocol.MethodRun, protocol.OpSet, protocol.PreferenceArgs{ExtensionID: "acme.units", Value: json.RawMessage(`1`)}), api.CodeInvalid, "setting id")
}

func TestPreferencesAreUnavailableWithoutAnEditor(t *testing.T) {
	reader := New(Deps{}, manifestFor("acme.reader", api.CapabilityPreferences))
	requireCode(t, servePreference(reader, protocol.MethodRun, protocol.OpList, protocol.QueryArgs{}), api.CodeUnavailable, "preferences")
}

// The user's own change needs no settings.write from the extension, and still refuses a value the setting does not
// accept.
func TestSavingASettingForTheUserChecksTheValueButNotSettingsWrite(t *testing.T) {
	manifest := boundedSettingsManifest("acme.bounded")
	manifest.Capabilities = nil
	h := handlerFor(t, newRig(t), manifest)
	if err := h.SaveSettingForUser("units", []byte(`"furlongs"`)); err == nil {
		t.Fatal("a value outside the choices was saved")
	}
	if err := h.SaveSettingForUser("missing", []byte(`1`)); err == nil {
		t.Fatal("an undeclared setting was saved")
	}
	if err := h.SaveSettingForUser("units", []byte(`"imperial"`)); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := decodeGet(t, mustOK(t, serve(t, h, protocol.ServiceSettings, protocol.OpGet, protocol.KeyArgs{Key: "units"}))); string(got.Value) != `"imperial"` {
		t.Fatalf("units = %s", got.Value)
	}
	requireCode(t, serve(t, h, protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: "units", Value: []byte(`"metric"`)}), api.CodePermission, "settings.write")
}
