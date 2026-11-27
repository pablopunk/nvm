package manager

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	dictbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
)

// TestPreferencesListAndChangeTheDictationSettings runs the preference editor over the real Dictation built-in: its
// declared settings are listed, the published microphone title is not, an engine it does not declare is refused, and
// turning AI cleanup on is saved.
func TestPreferencesListAndChangeTheDictationSettings(t *testing.T) {
	h := newHarness(t, func(opts *Options) { opts.BuiltIns = []api.Extension{dictbuiltin.New(dictbuiltin.Legacy{})} })
	h.waitRunning(dictbuiltin.ID)
	editor := NewPreferenceEditor(func() PreferenceSource { return h.m }, runAtOnce, nil, nil)

	records, err := editor.List(context.Background(), "dictation")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, record := range records {
		listed[record.ID] = true
	}
	for _, id := range []string{dictbuiltin.SettingDeviceID, dictbuiltin.SettingEngine, dictbuiltin.SettingCleanup,
		dictbuiltin.SettingDictionary, dictbuiltin.SettingClipboard, dictbuiltin.SettingScreen} {
		if !listed[id] {
			t.Errorf("List(dictation) left out %q: %+v", id, records)
		}
	}
	if listed[dictbuiltin.SettingDevice] {
		t.Errorf("List(dictation) listed the published microphone title")
	}

	_, err = editor.Plan(context.Background(), dictbuiltin.ID, dictbuiltin.SettingEngine, json.RawMessage(`"whisper"`))
	if !errors.Is(err, api.ErrInvalidInput) || !strings.Contains(err.Error(), "accepts only") {
		t.Fatalf("plan of an unknown engine: %v; want it refused", err)
	}

	if _, err := editor.Set(context.Background(), dictbuiltin.ID, dictbuiltin.SettingCleanup, json.RawMessage(`true`), ""); err != nil {
		t.Fatalf("turning AI cleanup on: %v", err)
	}
	if value, err := h.m.SettingValue(dictbuiltin.ID, dictbuiltin.SettingCleanup); err != nil || string(value) != "true" {
		t.Fatalf("cleanup = %s, %v; want it saved on", value, err)
	}
}
