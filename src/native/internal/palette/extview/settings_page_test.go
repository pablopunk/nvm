package extview

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// settingStore is the host's settings for one extension in a test: values by key, the writes it saw and the recorder
// calls it received.
type settingStore struct {
	values   map[string]json.RawMessage
	writes   []string
	recorded []string
	writeErr error
}

func (st *settingStore) deps(files *fake.FS) Deps {
	return Deps{
		Invoke:  func(string, api.Invocation, func(api.Result, *api.Failure)) {},
		Load:    func(string, string, func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {},
		OpenURL: func(string) {}, CopyText: func(search.Host, string) {}, Files: files,
		ReadSetting: func(_, key string) (json.RawMessage, error) {
			value, ok := st.values[key]
			if !ok {
				return nil, api.ErrNotFound
			}
			return value, nil
		},
		WriteSetting: func(_, key string, value json.RawMessage) error {
			if st.writeErr != nil {
				return st.writeErr
			}
			st.values[key] = value
			st.writes = append(st.writes, key)
			return nil
		},
		RecordSetting: func(_ search.Host, _, key string) { st.recorded = append(st.recorded, key) },
		RowAccessory: func(_, rowID string) (string, string, bool) {
			if rowID == "shortcut" {
				return "Not active", "", true
			}
			return "", "", false
		},
	}
}

func settingsSession(t *testing.T, st *settingStore) *Session {
	t.Helper()
	open := api.Action{Kind: api.ActionOpenURL, URL: "https://example.com"}
	view := api.SettingsView("Settings",
		api.SettingRow{ID: "compact", Kind: api.SettingRowToggle, Setting: "compact", Title: "Compact", ActionTitle: "Toggle",
			Also: []api.SettingRow{{ID: "preview", Kind: api.SettingRowToggle, Setting: "preview", Title: "Preview", ActionTitle: "Previews"}}},
		api.SettingRow{ID: "units", Kind: api.SettingRowChoice, Setting: "units", Title: "Units", ActionTitle: "Next",
			Choices: []api.SettingChoice{{Value: "metric", Title: "Metric"}, {Value: "imperial", Title: "Imperial"}}},
		api.SettingRow{ID: "shortcut", Kind: api.SettingRowRecorder, Setting: "shortcut", Title: "Shortcut"},
		api.SettingRow{ID: "label", Kind: api.SettingRowText, Setting: "label", Title: "Label"},
		api.SettingRow{ID: "site", Kind: api.SettingRowAction, Title: "Site", Action: &open},
	)
	return NewSession(st.deps(fake.NewFS()), "acme.settings", view)
}

func settingsRow(t *testing.T, page *palette.View, query, title string) search.Item {
	t.Helper()
	for _, row := range page.Search(query) {
		if row.Title == title {
			return row
		}
	}
	t.Fatalf("no row %q for query %q", title, query)
	return search.Item{}
}

func TestSettingsRowsShowTheStoredValueAtTheEdge(t *testing.T) {
	st := &settingStore{values: map[string]json.RawMessage{
		"compact": json.RawMessage(`true`), "units": json.RawMessage(`"imperial"`), "label": json.RawMessage(`"Work"`),
	}}
	page := settingsSession(t, st).Palette()
	cases := map[string]string{"Compact": "On", "Units": "Imperial", "Shortcut": "Not active", "Label": "Work", "Site": ""}
	for title, want := range cases {
		if got := settingsRow(t, page, "", title).Accessory; got != want {
			t.Fatalf("%s accessory = %q; want %q", title, got, want)
		}
	}
}

func TestSettingsSearchMatchesTitlesOnly(t *testing.T) {
	page := settingsSession(t, &settingStore{values: map[string]json.RawMessage{}}).Palette()
	if rows := page.Search("unit"); len(rows) != 1 || rows[0].Title != "Units" {
		t.Fatalf("search for unit = %v; want the Units row only", rowTitles(rows))
	}
	if rows := page.Search("metric"); len(rows) != 0 {
		t.Fatalf("a choice's title matched the search: %v", rowTitles(rows))
	}
}

func TestToggleAndChoiceSaveThroughTheHostAndRefresh(t *testing.T) {
	st := &settingStore{values: map[string]json.RawMessage{"compact": json.RawMessage(`false`), "units": json.RawMessage(`"imperial"`)}}
	page := settingsSession(t, st).Palette()

	host := &fakeHost{}
	settingsRow(t, page, "", "Compact").Actions[0].Run(host)
	if string(st.values["compact"]) != "true" || host.refresh != 1 {
		t.Fatalf("toggle stored %s with %d refreshes; want true and one refresh", st.values["compact"], host.refresh)
	}

	settingsRow(t, page, "", "Units").Actions[0].Run(host)
	if string(st.values["units"]) != `"metric"` {
		t.Fatalf("choice after the last stepped to %s; want the first choice", st.values["units"])
	}
}

func TestAlsoActionTogglesTheSecondSetting(t *testing.T) {
	st := &settingStore{values: map[string]json.RawMessage{"preview": json.RawMessage(`false`)}}
	page := settingsSession(t, st).Palette()
	row := settingsRow(t, page, "", "Compact")
	if len(row.Actions) != 2 || row.Actions[1].Title != "Previews" {
		t.Fatalf("actions = %v; want the toggle and Previews", row.Actions)
	}
	row.Actions[1].Run(&fakeHost{})
	if string(st.values["preview"]) != "true" {
		t.Fatalf("Previews stored %s; want true", st.values["preview"])
	}
}

func TestRecorderAsksTheHostAndFailedWritesToast(t *testing.T) {
	st := &settingStore{values: map[string]json.RawMessage{"compact": json.RawMessage(`false`)}, writeErr: errors.New("the setting cannot be saved")}
	page := settingsSession(t, st).Palette()
	host := &fakeHost{}

	settingsRow(t, page, "", "Shortcut").Actions[0].Run(host)
	if len(st.recorded) != 1 || st.recorded[0] != "shortcut" {
		t.Fatalf("recorder calls = %v; want the shortcut setting", st.recorded)
	}

	settingsRow(t, page, "", "Compact").Actions[0].Run(host)
	if len(host.toasts) != 1 || host.toasts[0] != "the setting cannot be saved" {
		t.Fatalf("toasts = %v; want the save failure", host.toasts)
	}
}

func TestTextRowPushesAnEditorThatSavesTheTypedText(t *testing.T) {
	st := &settingStore{values: map[string]json.RawMessage{"label": json.RawMessage(`"Work"`)}}
	page := settingsSession(t, st).Palette()
	host := &fakeHost{}
	settingsRow(t, page, "", "Label").Actions[0].Run(host)
	if len(host.pushed) != 1 {
		t.Fatalf("pushed %d views; want the editor", len(host.pushed))
	}
	editor := host.pushed[0]
	if rows := editor.Search(""); len(rows) != 0 {
		t.Fatalf("an empty editor offered %v; want nothing to save", rowTitles(rows))
	}
	save := editor.Search("Home")[0].Actions[0]
	save.Run(host)
	if string(st.values["label"]) != `"Home"` || host.pops != 1 {
		t.Fatalf("label = %s with %d pops; want Home saved and the editor closed", st.values["label"], host.pops)
	}
}

func TestActionRowRunsItsDeclaredAction(t *testing.T) {
	var opened string
	st := &settingStore{values: map[string]json.RawMessage{}}
	deps := st.deps(fake.NewFS())
	deps.OpenURL = func(url string) { opened = url }
	view := api.SettingsView("Settings", api.SettingRow{ID: "site", Kind: api.SettingRowAction, Title: "Site", Action: &api.Action{Kind: api.ActionOpenURL, URL: "https://example.com"}})
	NewSession(deps, "acme.settings", view).Palette().Search("")[0].Actions[0].Run(&fakeHost{})
	if opened != "https://example.com" {
		t.Fatalf("opened %q; want the declared URL", opened)
	}
}
