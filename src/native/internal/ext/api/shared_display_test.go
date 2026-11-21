package api

import "testing"

func sharedDisplayManifest(publishes []string, reads []SettingRef) Manifest {
	return Manifest{
		APIVersion: "1.0",
		Identity:   Identity{ID: "nevermind.dictation", Title: "Dictation"},
		Settings:   []SettingDefinition{{ID: "device", Type: SettingString}},
		Publishes:  publishes,
		Reads:      reads,
	}
}

func TestBuiltInManifestPublishesOnlyDeclaredSettings(t *testing.T) {
	if err := sharedDisplayManifest([]string{"device"}, nil).ValidateBuiltIn(); err != nil {
		t.Fatalf("declared publish refused: %v", err)
	}
	if err := sharedDisplayManifest([]string{"volume"}, nil).ValidateBuiltIn(); err == nil {
		t.Fatal("publish of an undeclared setting accepted")
	}
}

func TestSharedReadsNeedAValidReference(t *testing.T) {
	valid := SettingRef{Extension: "nevermind.dictation", Setting: "device"}
	if err := sharedDisplayManifest(nil, []SettingRef{valid}).ValidateBuiltIn(); err != nil {
		t.Fatalf("valid read refused: %v", err)
	}
	if err := sharedDisplayManifest(nil, []SettingRef{{Extension: "dictation", Setting: "device"}}).ValidateBuiltIn(); err == nil {
		t.Fatal("read of an extension id without the built-in namespace accepted")
	}
}

func TestFolderExtensionsCannotPublishOrRead(t *testing.T) {
	folder := sharedDisplayManifest([]string{"device"}, nil)
	folder.ID = "acme.dictation"
	if err := folder.Validate(); err == nil {
		t.Fatal("folder extension publishing a display accepted")
	}
}

func TestSettingRowShowsAValidSharedSetting(t *testing.T) {
	row := SettingRow{ID: "dictation", Kind: SettingRowAction, Title: "Dictation", Shows: &SettingRef{Extension: "nevermind.dictation", Setting: "device"},
		Action: &Action{Kind: ActionHost, Op: "dictation-settings", Title: "Open"}}
	if err := (SettingsSpec{Rows: []SettingRow{row}}).Validate(); err != nil {
		t.Fatalf("row showing a shared setting refused: %v", err)
	}
	row.Shows = &SettingRef{Extension: "nevermind.dictation"}
	if err := (SettingsSpec{Rows: []SettingRow{row}}).Validate(); err == nil {
		t.Fatal("row with an empty shared setting accepted")
	}
}
