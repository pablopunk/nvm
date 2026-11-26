package settings

import (
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestChoiceRowsStepThroughTheDeclaredChoices checks the manifest is consistent and every choice row offers exactly the
// choices its setting declares, so a write from the row is one the host accepts.
func TestChoiceRowsStepThroughTheDeclaredChoices(t *testing.T) {
	if err := New().Manifest().ValidateBuiltIn(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	for _, row := range pageRows(true) {
		if row.Kind != api.SettingRowChoice {
			continue
		}
		declared := declaredChoices(row.Setting)
		if len(declared) == 0 || !slices.Equal(row.Choices, declared) {
			t.Errorf("row %q choices = %v; want the declared %v", row.ID, row.Choices, declared)
		}
	}
}

func TestChoiceSettingsRefuseUnknownValues(t *testing.T) {
	refused := map[string]string{
		SettingUpdateChannel: `"beta"`, SettingMenuBarIconStyle: `"rainbow"`, SettingSearchEngine: `"altavista"`, SettingClipboardLimit: `"7"`,
	}
	for _, definition := range declaredSettings() {
		value, ok := refused[definition.ID]
		if ok && definition.CheckValue([]byte(value)) == nil {
			t.Errorf("setting %q accepted %s", definition.ID, value)
		}
	}
}
