package syspanes

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fakeContext serves the System service with a fixed list of panes; every other service is absent.
type fakeContext struct {
	api.Context
	panes []api.SettingsPane
}

func (c fakeContext) System() api.System { return fakeSystem{panes: c.panes} }

type fakeSystem struct {
	api.System
	panes []api.SettingsPane
}

func (s fakeSystem) SettingsPanes() []api.SettingsPane { return s.panes }

var samplePanes = []api.SettingsPane{
	{ID: "com.apple.Sound-Settings.extension", Title: "Sound"},
	{ID: "com.apple.Displays-Settings.extension", Title: "Displays"},
	{ID: "com.apple.Keyboard-Settings.extension", Title: "Keyboard"},
}

func TestEmptyQueryListsNoPanes(t *testing.T) {
	items, err := Extension{}.SearchItems(fakeContext{panes: samplePanes}, "")
	if err != nil || len(items) != 0 {
		t.Fatalf("items = %+v, err %v; want none for an empty query", items, err)
	}
}

func TestQueryMatchesPaneTitlesAndOpensThePane(t *testing.T) {
	items, err := Extension{}.SearchItems(fakeContext{panes: samplePanes}, "sou")
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Sound" {
		t.Fatalf("items = %+v, want the Sound pane only", items)
	}
	item := items[0]
	if item.Section != "Commands" || item.Subtitle != "System Settings" || item.Icon != "settings" || item.Score <= 0 {
		t.Errorf("row presentation = section %q subtitle %q icon %q score %v", item.Section, item.Subtitle, item.Icon, item.Score)
	}
	primary := item.Primary
	if primary == nil || primary.Kind != api.ActionSystem || primary.Op != "open-system-settings" || primary.Setting != samplePanes[0].ID {
		t.Errorf("primary = %+v, want the system op that opens the pane", primary)
	}
}

// TestAliasesFindThePaneTheElectronAppFinds checks the names the Electron app matches a pane by, which its titles do
// not contain: "iCloud" finds Apple Account and "Wireless" finds Wi-Fi.
func TestAliasesFindThePaneTheElectronAppFinds(t *testing.T) {
	panes := []api.SettingsPane{
		{ID: "com.apple.systempreferences.AppleIDSettings", Title: "Apple Account"},
		{ID: "com.apple.wifi-settings-extension", Title: "Wi-Fi"},
		{ID: "com.apple.Sound-Settings.extension", Title: "Sound"},
	}
	for query, want := range map[string]string{"icloud": "Apple Account", "wireless": "Wi-Fi", "apple id": "Apple Account"} {
		items, err := Extension{}.SearchItems(fakeContext{panes: panes}, query)
		if err != nil || len(items) != 1 || items[0].Title != want {
			t.Errorf("query %q: items = %+v, err %v; want only %s", query, items, err, want)
		}
	}
}

func TestNoPanesWhenTheOSHasNone(t *testing.T) {
	items, err := Extension{}.SearchItems(fakeContext{}, "sound")
	if err != nil || len(items) != 0 {
		t.Fatalf("items = %+v, err %v; want none", items, err)
	}
}

func TestManifestIsAValidBuiltIn(t *testing.T) {
	if err := (Extension{}).Manifest().ValidateBuiltIn(); err != nil {
		t.Fatalf("ValidateBuiltIn: %v", err)
	}
}
