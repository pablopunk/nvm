package api

import (
	"strings"
	"testing"
)

func validToggle(id, setting string) SettingRow {
	return SettingRow{ID: id, Kind: SettingRowToggle, Setting: setting, Title: "Toggle " + id}
}

func TestSettingsViewValidatesEachRowKind(t *testing.T) {
	open := Action{Kind: ActionOpenPath, Path: "/tmp"}
	valid := SettingsView("Settings",
		validToggle("compact", "compact"),
		SettingRow{ID: "units", Kind: SettingRowChoice, Setting: "units", Title: "Units", Choices: []SettingChoice{{Value: "metric", Title: "Metric"}, {Value: "imperial", Title: "Imperial"}}},
		SettingRow{ID: "shortcut", Kind: SettingRowRecorder, Setting: "shortcut", Title: "Shortcut"},
		SettingRow{ID: "label", Kind: SettingRowText, Setting: "label", Title: "Label"},
		SettingRow{ID: "folder", Kind: SettingRowAction, Title: "Folder", Action: &open},
	)
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid view: %v", err)
	}
}

func TestSettingsRowsRefuseMalformedShapes(t *testing.T) {
	open := Action{Kind: ActionOpenPath, Path: "/tmp"}
	cases := map[string]struct {
		rows []SettingRow
		want string
	}{
		"unknown kind":        {[]SettingRow{{ID: "a", Kind: "slider", Setting: "a", Title: "A"}}, "unknown kind"},
		"no setting":          {[]SettingRow{{ID: "a", Kind: SettingRowToggle, Title: "A"}}, "needs a setting"},
		"bad setting id":      {[]SettingRow{{ID: "a", Kind: SettingRowToggle, Setting: "1bad", Title: "A"}}, "valid setting id"},
		"choice with one":     {[]SettingRow{{ID: "a", Kind: SettingRowChoice, Setting: "a", Title: "A", Choices: []SettingChoice{{Value: "x", Title: "X"}}}}, "at least two"},
		"choice repeats":      {[]SettingRow{{ID: "a", Kind: SettingRowChoice, Setting: "a", Title: "A", Choices: []SettingChoice{{Value: "x", Title: "X"}, {Value: "x", Title: "Y"}}}}, "repeats"},
		"action without one":  {[]SettingRow{{ID: "a", Kind: SettingRowAction, Title: "A"}}, "needs an action"},
		"action with setting": {[]SettingRow{{ID: "a", Kind: SettingRowAction, Setting: "a", Title: "A", Action: &open}}, "stores no setting"},
		"duplicate id":        {[]SettingRow{validToggle("a", "a"), validToggle("a", "b")}, "used twice"},
		"missing title":       {[]SettingRow{{ID: "a", Kind: SettingRowToggle, Setting: "a"}}, "needs a title"},
		"nested also":         {[]SettingRow{{ID: "a", Kind: SettingRowToggle, Setting: "a", Title: "A", Also: []SettingRow{{ID: "b", Kind: SettingRowToggle, Setting: "b", Title: "B", Also: []SettingRow{validToggle("c", "c")}}}}}, "extra action cannot"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := SettingsSpec{Rows: tc.rows}.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() = %v; want an error mentioning %q", err, tc.want)
			}
		})
	}
}

// TestSettingsViewIsValidatedThroughView checks the settings spec is reached from View.Validate, and that a settings
// view carries only its own spec.
func TestSettingsViewIsValidatedThroughView(t *testing.T) {
	bad := SettingsView("Settings", SettingRow{ID: "a", Kind: SettingRowToggle, Title: "A"})
	if err := bad.Validate(); err == nil {
		t.Fatal("a settings view with an invalid row validated")
	}
	mixed := SettingsView("Settings", validToggle("a", "a"))
	mixed.Form = &FormSpec{}
	if err := mixed.Validate(); err == nil {
		t.Fatal("a settings view with a form spec validated")
	}
}
