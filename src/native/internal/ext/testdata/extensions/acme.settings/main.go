package main

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.settings is the dev fixture of the settings rows. The root row opens a settings view whose rows are a toggle, a
// choice, a recorder, a text row and an action row, each over one of the extension's declared settings. The host draws
// and saves them; the extension only declares them.

const rootTitle = "Settings fixture: rows"

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if !strings.Contains(strings.ToLower(rootTitle), strings.ToLower(query)) {
		return nil, nil
	}
	return []api.Item{{ID: "open", Title: rootTitle, Primary: api.Primary(api.Run("open", "Open", nil))}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "open" {
		return api.Result{}, nil
	}
	view := api.SettingsView("Settings fixture", fixtureRows()...)
	view.Placeholder = "Search Settings"
	return api.Result{View: &view}, nil
}

func fixtureRows() []api.SettingRow {
	site := api.Action{Kind: api.ActionOpenURL, URL: "https://example.com"}
	return []api.SettingRow{
		{ID: "compact", Kind: api.SettingRowToggle, Setting: "compact", Title: "Compact", ActionTitle: "Toggle"},
		{ID: "units", Kind: api.SettingRowChoice, Setting: "units", Title: "Units", ActionTitle: "Next", Choices: []api.SettingChoice{
			{Value: "metric", Title: "Metric"}, {Value: "imperial", Title: "Imperial"},
		}},
		{ID: "shortcut", Kind: api.SettingRowRecorder, Setting: "shortcut", Title: "Shortcut"},
		{ID: "label", Kind: api.SettingRowText, Setting: "label", Title: "Label", ActionTitle: "Edit"},
		{ID: "site", Kind: api.SettingRowAction, Title: "Site", Action: &site},
	}
}
