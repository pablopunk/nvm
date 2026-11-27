package dictation

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// The actions of the settings page that the host's rows do not run themselves.
const (
	refEditDictionary  = "edit-dictionary"
	refSaveDictionary  = "save-dictionary"
	refInstallModel    = "install-device-model"
	dictionaryHint     = "One preferred term per line"
	settingsSearchHint = "Search Dictation Settings"
)

// openSettings shows the settings page, with the microphones, the model and the AI state read now. Opening it also
// publishes the microphone's title, so the Settings page's Dictation row names the choice made here.
func (e Extension) openSettings(ctx api.Context) (api.Result, error) {
	settings := readSettings(ctx)
	publishDeviceTitle(ctx, settings.DeviceID)
	return pushView(ctx, settingsView(ctx, settings)), nil
}

// settingsView is the Dictation settings page. The host draws each row over its declared setting and saves each change.
func settingsView(ctx api.Context, settings Settings) api.View {
	model, _ := ctx.Dictation().DeviceModel()
	var rows []api.SettingRow
	if microphone, offered := microphoneRow(devicesOf(ctx)); offered {
		rows = append(rows, microphone)
	}
	rows = append(rows,
		engineRow(model),
		cleanupRow(aiAvailable(ctx)),
		dictionaryRow(),
		toggleRow(SettingClipboard, "Copy Transcription to Clipboard", "clipboard", "Keep the transcription on the clipboard after pasting."),
		toggleRow(SettingScreen, "Use Screen Text", "scan-text", "Read words from the screen to fix names and terms. Off unless you turn it on."),
	)
	view := api.SettingsView(settingsTitle, rows...)
	view.Placeholder = settingsSearchHint
	return view
}

// microphoneRow steps through the system microphone and each listed one. With no microphone listed there is nothing
// to choose, so no row is offered.
func microphoneRow(devices []api.DictationDevice) (api.SettingRow, bool) {
	choices := []api.SettingChoice{{Value: defaultDeviceID, Title: systemDefaultTitle}}
	for _, device := range devices {
		if !isDefaultDevice(device.ID) {
			choices = append(choices, api.SettingChoice{Value: device.ID, Title: device.Title})
		}
	}
	row := api.SettingRow{ID: SettingDeviceID, Kind: api.SettingRowChoice, Setting: SettingDeviceID, Title: "Microphone",
		Icon: "mic", ActionTitle: "Next Microphone", Choices: choices}
	return row, len(choices) > 1
}

// engineRow chooses where speech becomes text and says where the on-device model's download stands. Until the model
// is downloaded, the row also offers to download it.
func engineRow(model api.DictationDeviceModel) api.SettingRow {
	row := api.SettingRow{ID: SettingEngine, Kind: api.SettingRowChoice, Setting: SettingEngine, Title: "Transcription",
		Subtitle: deviceModelDescription(model), Icon: "monitor", ActionTitle: "Switch", Choices: engineChoices}
	if model.Status != api.DictationModelReady && model.Status != api.DictationModelDownloading {
		install := api.Run(refInstallModel, "Download On-Device Model", nil)
		row.Also = []api.SettingRow{{ID: refInstallModel, Kind: api.SettingRowAction, Title: install.Title, Action: &install}}
	}
	return row
}

// cleanupRow is the AI cleaning switch. Without an account it says so; dictation cleans only when the account can.
func cleanupRow(available bool) api.SettingRow {
	subtitle := "Sign in to Nevermind to use AI cleaning."
	if available {
		subtitle = "Send the transcript to Fast AI to correct punctuation, grammar, and transcription errors before pasting."
	}
	return api.SettingRow{ID: SettingCleanup, Kind: api.SettingRowToggle, Setting: SettingCleanup, Title: "Clean with AI",
		Subtitle: subtitle, Icon: "sparkles", ActionTitle: "Toggle"}
}

// dictionaryRow opens the dictionary in an editor, since it is long-form text with one term per line.
func dictionaryRow() api.SettingRow {
	edit := api.Run(refEditDictionary, "Edit", nil)
	return api.SettingRow{ID: SettingDictionary, Kind: api.SettingRowAction, Title: "Custom Dictionary",
		Subtitle: "Preferred terms and spellings for AI cleanup.", Icon: "text", Action: &edit}
}

func toggleRow(setting, title, icon, subtitle string) api.SettingRow {
	return api.SettingRow{ID: setting, Kind: api.SettingRowToggle, Setting: setting, Title: title, Subtitle: subtitle,
		Icon: icon, ActionTitle: "Toggle"}
}

// editDictionary opens the dictionary in an editor. Saving stores the text as the dictionary setting.
func editDictionary(ctx api.Context) (api.Result, error) {
	view := api.EditorView("Custom Dictionary", readSettings(ctx).Dictionary, api.Run(refSaveDictionary, "Save", nil))
	view.Editor.Placeholder = dictionaryHint
	return pushView(ctx, view), nil
}

// saveDictionary stores the editor's text and goes back to the settings page.
func saveDictionary(ctx api.Context, text string) (api.Result, error) {
	if err := ctx.Settings().Set(SettingDictionary, text); err != nil {
		return api.Result{}, err
	}
	return ctx.Navigation().Pop(), nil
}

// installDeviceModel starts the on-device model's download and says so.
func installDeviceModel(ctx api.Context) (api.Result, error) {
	toast := installNotice(ctx, api.DictationEngineDevice)
	if toast == nil {
		toast = &api.Toast{Message: "The on-device model is downloaded or on its way"}
	}
	return api.Result{Toast: toast}, nil
}
