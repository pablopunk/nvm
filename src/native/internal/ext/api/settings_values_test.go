package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func settingBound(value float64) *float64 {
	return &value
}

func unitsSetting() SettingDefinition {
	return SettingDefinition{ID: "units", Type: SettingString, Value: json.RawMessage(`"metric"`),
		Choices: []SettingChoice{{Value: "metric", Title: "Metric"}, {Value: "imperial", Title: "Imperial"}}}
}

func gapSetting() SettingDefinition {
	return SettingDefinition{ID: "gap", Type: SettingNumber, Value: json.RawMessage(`8`), Min: settingBound(0), Max: settingBound(40)}
}

func TestCheckValueAcceptsDeclaredValuesOnly(t *testing.T) {
	optionalGap := gapSetting()
	optionalGap.Type = ""
	cases := map[string]struct {
		definition SettingDefinition
		value      string
		refusal    string
	}{
		"choice":              {unitsSetting(), `"imperial"`, ""},
		"unknown choice":      {unitsSetting(), `"furlongs"`, `accepts only "metric", "imperial"`},
		"wrong type":          {unitsSetting(), `3`, "expects a string"},
		"invalid json":        {unitsSetting(), `{`, "not valid JSON"},
		"number in range":     {gapSetting(), `40`, ""},
		"number above max":    {gapSetting(), `41`, "from 0 to 40"},
		"number below min":    {gapSetting(), `-1`, "from 0 to 40"},
		"untyped null":        {optionalGap, `null`, ""},
		"untyped out of":      {optionalGap, `99`, "from 0 to 40"},
		"min only":            {SettingDefinition{ID: "n", Type: SettingNumber, Min: settingBound(1)}, `0`, "at least 1"},
		"max only":            {SettingDefinition{ID: "n", Type: SettingNumber, Max: settingBound(5)}, `6`, "at most 5"},
		"untyped without any": {SettingDefinition{ID: "any"}, `{"a":1}`, ""},
	}
	for name, tc := range cases {
		err := tc.definition.CheckValue(json.RawMessage(tc.value))
		if tc.refusal == "" && err != nil {
			t.Errorf("%s: refused %s: %v", name, tc.value, err)
		}
		if tc.refusal != "" && (err == nil || !strings.Contains(err.Error(), tc.refusal)) {
			t.Errorf("%s: error = %v, want it to mention %q", name, err, tc.refusal)
		}
	}
}

func TestManifestRefusesInconsistentSettingValues(t *testing.T) {
	withValue := func(definition SettingDefinition, value string) SettingDefinition {
		definition.Value = json.RawMessage(value)
		return definition
	}
	reversed := gapSetting()
	reversed.Min, reversed.Max = settingBound(10), settingBound(2)
	reversed.Value = nil
	cases := map[string]struct {
		definition SettingDefinition
		refusal    string
	}{
		"choices on a boolean":   {SettingDefinition{ID: "b", Type: SettingBoolean, Choices: unitsSetting().Choices}, "need type string"},
		"choices on a number":    {SettingDefinition{ID: "n", Type: SettingNumber, Choices: unitsSetting().Choices}, "need type string"},
		"choice without a title": {SettingDefinition{ID: "s", Type: SettingString, Choices: []SettingChoice{{Value: "a"}}}, "value and a title"},
		"repeated choice":        {SettingDefinition{ID: "s", Type: SettingString, Choices: []SettingChoice{{Value: "a", Title: "A"}, {Value: "a", Title: "B"}}}, "repeats"},
		"range on a string":      {SettingDefinition{ID: "s", Type: SettingString, Min: settingBound(1)}, "need type number"},
		"min above max":          {reversed, "min 10 above max 2"},
		"default outside choice": {withValue(unitsSetting(), `"furlongs"`), "default"},
		"default outside range":  {withValue(gapSetting(), `99`), "default"},
		"default of wrong type":  {withValue(gapSetting(), `"8"`), "default"},
	}
	for name, tc := range cases {
		manifest := Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.units", Title: "Units"}, Settings: []SettingDefinition{tc.definition}}
		if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), tc.refusal) {
			t.Errorf("%s: error = %v, want it to mention %q", name, err, tc.refusal)
		}
	}
	consistent := Manifest{APIVersion: "1.0", Identity: Identity{ID: "acme.units", Title: "Units"},
		Settings: []SettingDefinition{unitsSetting(), gapSetting(), {ID: "blank", Min: settingBound(0), Value: json.RawMessage(`null`)}}}
	if err := consistent.Validate(); err != nil {
		t.Fatalf("consistent settings refused: %v", err)
	}
}
