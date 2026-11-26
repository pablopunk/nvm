package codemode

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// fakeSettingsSource is the extension manager's side of the settings, under the host's real preference editor: the
// Settings page's clipboard size and palette shortcut, and their saved values.
type fakeSettingsSource struct {
	mu     sync.Mutex
	values map[string]json.RawMessage
}

func (s *fakeSettingsSource) DeclaredPreferences() []manager.ExtensionPreferences {
	sizes := []api.SettingChoice{{Value: "300", Title: "300 items"}, {Value: "500", Title: "500 items"}, {Value: "1000", Title: "1000 items"}}
	return []manager.ExtensionPreferences{{ID: settings.ID, Title: "Settings", Settings: []api.SettingDefinition{
		{ID: settings.SettingPaletteShortcut, Title: "Open Nevermind Shortcut", Type: api.SettingString, Value: json.RawMessage(`""`)},
		{ID: settings.SettingClipboardLimit, Title: "Clipboard History Size", Type: api.SettingString, Value: json.RawMessage(`"300"`), Choices: sizes},
	}}}
}

func (s *fakeSettingsSource) SettingValue(id, key string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	if key == settings.SettingClipboardLimit {
		return json.RawMessage(`"300"`), nil
	}
	return json.RawMessage(`""`), nil
}

func (s *fakeSettingsSource) SaveSettingForUser(_, key string, value json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
	return nil
}

func (s *fakeSettingsSource) clipboardLimit() string {
	value, _ := s.SettingValue(settings.ID, settings.SettingClipboardLimit)
	return string(value)
}

func preferenceFixture(t *testing.T) (*fixture, *fakeSettingsSource) {
	source := &fakeSettingsSource{values: map[string]json.RawMessage{}}
	editor := manager.NewPreferenceEditor(func() manager.PreferenceSource { return source }, func(fn func()) { fn() }, nil, settings.RecordedElsewhere())
	return newFixture(t, func(opts *Options) { opts.Preferences = editor }), source
}

const setClipboardSize = `for _, p := range ctx.Preferences().List("clipboard size") {
	saved, err := ctx.Preferences().Set(p.ExtensionID, p.ID, "500")
	return saved.Value, err
}
return "no such setting", nil`

func TestPreferenceOperationsAreClassifiedByEffect(t *testing.T) {
	rules := preferenceRules()
	if len(rules) != 2 || rules[key{protocol.ServicePreferences, protocol.OpList}].class != ClassRead ||
		rules[key{protocol.ServicePreferences, protocol.OpSet}].class != ClassSetting {
		t.Fatalf("rules = %+v", rules)
	}
}

// A setting change asks with the extension, the setting and both values in the words of its choices, and runs once
// allowed.
func TestChangingASettingAsksWithChoiceTitlesAndRunsWhenAllowed(t *testing.T) {
	f, source := preferenceFixture(t)
	f.steps.decide = allow
	result := f.run(setClipboardSize)
	if result.Failed || source.clipboardLimit() != `"500"` {
		t.Fatalf("result = %+v, size %s", result, source.clipboardLimit())
	}
	row := lastRow(t, f, "Changing a setting")
	t.Logf("question: %s\nmodel reads: %s", row.detail, result.Text)
	if !row.asked || row.detail != "Change “Clipboard History Size” in Settings from “300 items” to “500 items”" {
		t.Fatalf("row = %+v", row)
	}
	contains(t, result.Text, "changed “Clipboard History Size” in Settings from “300 items” to “500 items” (the user allowed it)")
}

func TestADeclinedSettingChangeSavesNothing(t *testing.T) {
	f, source := preferenceFixture(t)
	f.steps.decide = decline
	result := f.run(setClipboardSize)
	if !result.Failed || source.clipboardLimit() != `"300"` {
		t.Fatalf("result = %+v, size %s", result, source.clipboardLimit())
	}
	contains(t, result.Text, "change “Clipboard History Size” in Settings from “300 items” to “500 items”: the user declined")
}

// A change the host refuses while preparing it fails with the host's reason, and the user is never asked.
func TestARefusedSettingChangeFailsWithoutAsking(t *testing.T) {
	cases := map[string]struct{ body, reason string }{
		"not a choice":       {`_, err := ctx.Preferences().Set("nevermind.settings", "clipboardLimit", "999")` + "\nreturn nil, err", "accepts only"},
		"same value":         {`_, err := ctx.Preferences().Set("nevermind.settings", "clipboardLimit", "300")` + "\nreturn nil, err", "already has this value"},
		"unknown setting":    {`_, err := ctx.Preferences().Set("nevermind.settings", "volume", 3)` + "\nreturn nil, err", "has no setting"},
		"recorded elsewhere": {`_, err := ctx.Preferences().Set("nevermind.settings", "paletteShortcut", "Command+K")` + "\nreturn nil, err", "Shortcuts().Set"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, source := preferenceFixture(t)
			f.steps.decide = allow
			result := f.run(c.body)
			t.Logf("model reads: %s", result.Text)
			if !result.Failed || !strings.Contains(result.Text, c.reason) {
				t.Fatalf("result = %+v", result)
			}
			if row := lastRow(t, f, "Changing a setting"); row.asked {
				t.Fatalf("a refused change was put to the user: %+v", row)
			}
			if source.clipboardLimit() != `"300"` {
				t.Fatalf("a refused change saved %s", source.clipboardLimit())
			}
		})
	}
}

// The approval covers the value the user saw: a value the user changed while the question waited is not overwritten.
func TestASettingApprovalDoesNotOverwriteAChangeMadeWhileItWaited(t *testing.T) {
	f, source := preferenceFixture(t)
	f.steps.decide = func(*fakeRow, context.Context) (bool, error) {
		_ = source.SaveSettingForUser(settings.ID, settings.SettingClipboardLimit, json.RawMessage(`"1000"`))
		return true, nil
	}
	result := f.run(setClipboardSize)
	if !result.Failed || !strings.Contains(result.Text, "changed since") || source.clipboardLimit() != `"1000"` {
		t.Fatalf("result = %+v, size %s", result, source.clipboardLimit())
	}
}

func TestListingSettingsDoesNotAsk(t *testing.T) {
	f, _ := preferenceFixture(t)
	result := f.run(`return len(ctx.Preferences().List("")), nil`)
	if result.Failed || !strings.Contains(result.Text, "Result: 1") {
		t.Fatalf("result = %+v; want the one setting a value can set", result)
	}
	for _, row := range f.steps.snapshot() {
		if row.asked {
			t.Fatalf("a read asked: %+v", row)
		}
	}
}
