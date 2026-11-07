package settings

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
)

// Row ids the host draws its own text for: the shortcut's status, the account, the updates and the config folder path.
// The rows themselves are declared here.
const (
	RowDictation    = "dictation"
	RowUpdates      = "updates"
	RowAccount      = "account"
	RowConfigFolder = "configFolder"
)

// Host actions the rows run, named by ActionHost ops.
const (
	opDictationSettings = "dictation-settings"
	opUpdates           = "updates"
	opAccountPage       = "account-page"
	opConfigFolder      = "config-folder"
)

// clipboardLimits are the sizes the clipboard history size row steps through.
var clipboardLimits = []string{"50", "300", "500", "1000"}

// updateChannels are the channels the update channel row steps through.
var updateChannels = []api.SettingChoice{{Value: "stable", Title: "Stable"}, {Value: "nightly", Title: "Nightly"}, {Value: "dev", Title: "Dev"}}

// menuBarStyles are the menu bar icon styles, with their 1.x labels.
var menuBarStyles = []api.SettingChoice{
	{Value: "bolt-filled", Title: "Filled Bolt"}, {Value: "bolt-outline", Title: "Bolt Outline"}, {Value: "emoji", Title: "Emoji ⚡"},
	{Value: "app-icon", Title: "App Icon"}, {Value: "gold-outline", Title: "Gold Bolt Outline"}, {Value: "gold-filled", Title: "Gold Bolt"},
	{Value: "gold-glossy", Title: "Glossy Gold Bolt"},
}

// pageRows are the rows of the settings page, in the order the native page showed them. The Launch at Login row is
// listed only where the OS can start the app at sign-in.
func pageRows(launchAtLoginSupported bool) []api.SettingRow {
	rows := []api.SettingRow{
		{ID: SettingPaletteShortcut, Kind: api.SettingRowRecorder, Setting: SettingPaletteShortcut, Title: "Open Nevermind Shortcut",
			Subtitle: "Optional global keyboard shortcut to open the palette", Icon: "keyboard", ActionTitle: "Change…"},
		{ID: SettingHyperKey, Kind: api.SettingRowRecorder, Setting: SettingHyperKey, Title: "Hyper Key",
			Subtitle: "Key combination displayed as Hyper in shortcut labels", Icon: "command", ActionTitle: "Change…"},
		{ID: SettingSearchEngine, Kind: api.SettingRowChoice, Setting: SettingSearchEngine, Title: "Default Search Engine", Icon: "search",
			ActionTitle: "Next Engine", Choices: engineChoices()},
		{ID: SettingClipboardHistory, Kind: api.SettingRowToggle, Setting: SettingClipboardHistory, Title: "Clipboard History", Icon: "clipboard",
			ActionTitle: "Toggle", Also: []api.SettingRow{{ID: SettingClipboardLinkPreviews, Kind: api.SettingRowToggle, Setting: SettingClipboardLinkPreviews, Title: "Link Previews", ActionTitle: "Link Previews"}}},
		{ID: SettingClipboardLimit, Kind: api.SettingRowChoice, Setting: SettingClipboardLimit, Title: "Clipboard History Size", Icon: "clipboard",
			ActionTitle: "Cycle", Choices: choicesOf(clipboardLimits)},
		{ID: SettingClipboardRootList, Kind: api.SettingRowToggle, Setting: SettingClipboardRootList, Title: "Show Clipboard Items in Root List",
			Subtitle: "Show copied items inline in the root list", Icon: "clipboard", ActionTitle: "Toggle"},
		{ID: SettingClipboardRootSearch, Kind: api.SettingRowToggle, Setting: SettingClipboardRootSearch, Title: "Show Clipboard Items in Root Search",
			Subtitle: "Show copied items when searching from the root list", Icon: "clipboard", ActionTitle: "Toggle"},
		{ID: RowDictation, Kind: api.SettingRowAction, Title: "Dictation", Icon: "mic", Action: hostAction(opDictationSettings, "Open"),
			Shows: sharedDictationDevice()},
	}
	if launchAtLoginSupported {
		rows = append(rows, api.SettingRow{ID: SettingLaunchAtLogin, Kind: api.SettingRowToggle, Setting: SettingLaunchAtLogin, Title: "Launch at Login",
			Icon: "power", ActionTitle: "Toggle"})
	}
	return append(rows,
		api.SettingRow{ID: SettingDiagnostics, Kind: api.SettingRowToggle, Setting: SettingDiagnostics, Title: "Send Technical Diagnostics",
			Subtitle: "Errors, usage and performance, without prompts or clipboard content", Icon: "shield-check", ActionTitle: "Toggle"},
		api.SettingRow{ID: RowUpdates, Kind: api.SettingRowAction, Title: "Updates", Icon: "rotate-ccw", Action: hostAction(opUpdates, "Open")},
		api.SettingRow{ID: SettingUpdateChannel, Kind: api.SettingRowChoice, Setting: SettingUpdateChannel, Title: "Update Channel", Icon: "shield-check",
			ActionTitle: "Switch", Choices: updateChannels},
		api.SettingRow{ID: SettingMenuBarIconStyle, Kind: api.SettingRowChoice, Setting: SettingMenuBarIconStyle, Title: "Menu Bar Icon Style",
			Subtitle: "How the Nevermind icon looks in the menu bar", Icon: "zap", ActionTitle: "Next", Choices: menuBarStyles},
		api.SettingRow{ID: RowAccount, Kind: api.SettingRowAction, Title: "Account", Icon: "user", Action: hostAction(opAccountPage, "Open")},
		api.SettingRow{ID: RowConfigFolder, Kind: api.SettingRowAction, Title: "Open Config Folder", Icon: "folder", Action: hostAction(opConfigFolder, "Open")},
	)
}

// sharedDictationDevice is the setting the dictation built-in publishes the microphone's title under. The Dictation row
// shows it, so the choice reads here without the Settings built-in knowing how dictation stores it.
func sharedDictationDevice() *api.SettingRef {
	return &api.SettingRef{Extension: dictation.ID, Setting: dictation.SettingDevice}
}

func hostAction(op, title string) *api.Action {
	return &api.Action{Kind: api.ActionHost, Op: op, Title: title}
}

// choicesOf makes choices whose values and titles are the same strings.
func choicesOf(values []string) []api.SettingChoice {
	choices := make([]api.SettingChoice, 0, len(values))
	for _, value := range values {
		choices = append(choices, api.SettingChoice{Value: value, Title: value})
	}
	return choices
}
