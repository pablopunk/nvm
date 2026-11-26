// Package settings is the Settings built-in: the app's settings page, drawn by the host from the rows this built-in
// declares. The values live in the host's store, so config.json and the app's own state stay where they were.
//
// Layer: ext. The package imports api, websearch and the standard library.
package settings

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
)

// ID is the built-in's identity. The Settings command in the root calls its open action.
const ID = "nevermind.settings"

// openAction is the action the root's Settings command runs to get the page.
const openAction = "open"

// Setting ids. Each row stores one of them; the host store maps an id to the value the app keeps.
const (
	SettingPaletteShortcut       = "paletteShortcut"
	SettingHyperKey              = "hyperKey"
	SettingSearchEngine          = "searchEngine"
	SettingClipboardHistory      = "clipboardHistory"
	SettingClipboardLinkPreviews = "clipboardLinkPreviews"
	SettingClipboardLimit        = "clipboardLimit"
	SettingClipboardRootList     = "clipboardRootList"
	SettingClipboardRootSearch   = "clipboardRootSearch"
	SettingAIChatsRootList       = "aiChatsRootList"
	SettingDiagnostics           = "diagnostics"
	SettingUpdateChannel         = "updateChannel"
	SettingLaunchAtLogin         = "launchAtLogin"
	SettingMenuBarIconStyle      = "menuBarIconStyle"
)

// launchAtLoginCapability is the platform capability that says the OS can start the app at sign-in.
const launchAtLoginCapability = "autostart.signIn"

// Extension is the Settings built-in. It keeps no state: every value is in the host store.
type Extension struct{}

// New returns the Settings built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. Its rows write settings, which the host store saves.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "Settings"},
		Capabilities: []api.Capability{api.CapabilitySettingsWrite},
		Settings:     declaredSettings(),
		Reads:        []api.SettingRef{*sharedDictationDevice()},
	}
}

func declaredSettings() []api.SettingDefinition {
	return []api.SettingDefinition{
		{ID: SettingPaletteShortcut, Title: "Open Nevermind Shortcut", Type: api.SettingString, Value: []byte(`""`)},
		{ID: SettingHyperKey, Title: "Hyper Key", Type: api.SettingString, Value: []byte(`""`)},
		{ID: SettingSearchEngine, Title: "Default Search Engine", Type: api.SettingString, Value: []byte(`"google"`), Choices: engineChoices()},
		{ID: SettingClipboardHistory, Title: "Clipboard History", Type: api.SettingBoolean, Value: []byte(`true`)},
		{ID: SettingClipboardLinkPreviews, Title: "Link Previews", Type: api.SettingBoolean, Value: []byte(`true`)},
		{ID: SettingClipboardLimit, Title: "Clipboard History Size", Type: api.SettingString, Value: []byte(`"300"`), Choices: choicesOf(clipboardLimits)},
		{ID: SettingClipboardRootList, Title: "Show Clipboard Items in Root List", Type: api.SettingBoolean, Value: []byte(`true`)},
		{ID: SettingClipboardRootSearch, Title: "Show Clipboard Items in Root Search", Type: api.SettingBoolean, Value: []byte(`true`)},
		{ID: SettingAIChatsRootList, Title: "Show AI Chats in Root List", Type: api.SettingBoolean, Value: []byte(`true`)},
		{ID: SettingDiagnostics, Title: "Send Technical Diagnostics", Type: api.SettingBoolean, Value: []byte(`true`)},
		{ID: SettingUpdateChannel, Title: "Update Channel", Type: api.SettingString, Value: []byte(`"stable"`), Choices: updateChannels},
		{ID: SettingLaunchAtLogin, Title: "Launch at Login", Type: api.SettingBoolean, Value: []byte(`false`)},
		{ID: SettingMenuBarIconStyle, Title: "Menu Bar Icon Style", Type: api.SettingString, Value: []byte(`"bolt-filled"`), Choices: menuBarStyles},
	}
}

// Commands lists nothing: the root's Settings command is the host's, and it calls the open action below.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	return nil, nil
}

// RunAction opens the settings page.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != openAction {
		return api.Result{}, nil
	}
	view := api.SettingsView("Settings", pageRows(ctx.System().HasCapability(launchAtLoginCapability))...)
	view.Placeholder = "Search Settings"
	return api.Result{View: &view}, nil
}

// engineChoices are the default search engines, in the order their row steps through them.
func engineChoices() []api.SettingChoice {
	return websearch.EngineChoices()
}

// declaredChoices are the choices the definition of setting id declares, which its choice row steps through.
func declaredChoices(id string) []api.SettingChoice {
	for _, definition := range declaredSettings() {
		if definition.ID == id {
			return definition.Choices
		}
	}
	return nil
}
