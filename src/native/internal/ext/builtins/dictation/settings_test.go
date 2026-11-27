package dictation

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestEveryRowOfTheSettingsPageStoresADeclaredSetting(t *testing.T) {
	manifest := New(Legacy{}).Manifest()
	if err := manifest.ValidateBuiltIn(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	ctx := newContext()
	ctx.dictation.devices = append(ctx.dictation.devices, api.DictationDevice{ID: "mic-2", Title: "Desk Mic"})
	view := run(t, New(Legacy{}), ctx, RefOpenSettings, nil, nil).View
	if view == nil || view.Settings == nil {
		t.Fatalf("view %+v; want a settings page", view)
	}
	if err := view.Settings.Validate(); err != nil {
		t.Fatalf("settings page invalid: %v", err)
	}
	stored := map[string]bool{}
	for _, row := range view.Settings.Rows {
		if row.Setting == "" {
			continue
		}
		if _, found := declared(row.Setting); !found {
			t.Errorf("row %q stores %q, which the manifest does not declare", row.ID, row.Setting)
		}
		stored[row.Setting] = true
	}
	for _, id := range []string{SettingDeviceID, SettingEngine, SettingCleanup, SettingClipboard, SettingScreen} {
		if !stored[id] {
			t.Errorf("no row stores %q", id)
		}
	}
}

func TestTheMicrophoneRowOffersTheSystemMicrophoneAndEachListedOne(t *testing.T) {
	row, offered := microphoneRow([]api.DictationDevice{{ID: "mic-1", Title: "USB Mic", IsDefault: true}, {ID: "mic-2", Title: "Desk Mic"}})
	want := []api.SettingChoice{{Value: defaultDeviceID, Title: systemDefaultTitle}, {Value: "mic-1", Title: "USB Mic"}, {Value: "mic-2", Title: "Desk Mic"}}
	if !offered || len(row.Choices) != len(want) {
		t.Fatalf("row %+v; want %v", row, want)
	}
	for i := range want {
		if row.Choices[i] != want[i] {
			t.Fatalf("choice %d = %+v; want %+v", i, row.Choices[i], want[i])
		}
	}
	if _, offered := microphoneRow(nil); offered {
		t.Fatal("a microphone row was offered with no microphone to choose")
	}
}

func TestTheEngineSettingRefusesAnUnknownEngine(t *testing.T) {
	ctx := newContext()
	if err := ctx.settings.Set(SettingEngine, "whisper"); err == nil {
		t.Fatal("the engine setting accepted an engine it does not declare")
	}
	ctx.settings.choose(t, map[string]any{SettingEngine: string(api.DictationEngineDevice)})
	if got := readSettings(ctx).Engine; got != api.DictationEngineDevice {
		t.Fatalf("engine %q; want the device", got)
	}
}

func TestANewInstallStartsOnTheCloudWithTheSystemMicrophoneAndNoCleaning(t *testing.T) {
	got := readSettings(newContext())
	want := Settings{DeviceID: defaultDeviceID, Engine: api.DictationEngineCloud}
	if got != want {
		t.Fatalf("settings %+v; want %+v", got, want)
	}
}

func TestTheDictionaryEditorSavesTheDictionarySetting(t *testing.T) {
	ctx := newContext()
	ext := New(Legacy{})
	view := run(t, ext, ctx, refEditDictionary, nil, nil).View
	if view == nil || view.Editor == nil || view.Editor.Submit == nil || view.Editor.Submit.Ref.ID != refSaveDictionary {
		t.Fatalf("view %+v; want an editor that saves the dictionary", view)
	}
	result, err := ext.RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refSaveDictionary}, EditorContent: "Kubernetes\nGrafana"})
	if err != nil || result.Navigation != api.NavPop {
		t.Fatalf("result %+v, %v; want the editor closed", result, err)
	}
	if got := readSettings(ctx).Dictionary; got != "Kubernetes\nGrafana" {
		t.Fatalf("dictionary %q; want the editor's text", got)
	}
}
