package dictation

import (
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/cleanup"
)

// storageSettingsKey and storageHistoryKey name the built-in's documents in its storage.
const (
	storageSettingsKey = "settings"
	storageHistoryKey  = "history"
)

// The settings form's fields, by id, as 1.x named them.
const (
	fieldDevice       = "deviceId"
	fieldCleanup      = "cleanupWithAi"
	fieldDictionary   = "dictionary"
	fieldClipboard    = "copyToClipboard"
	fieldScreen       = "useScreenContext"
	fieldScreenLimit  = "screenTermLimit"
	defaultDeviceID   = "default"
	settingsSave      = "Save"
	dictionaryRows    = 8
	dictionaryHint    = "One preferred term per line"
	dictionaryDetails = "Preferred terms and spellings for AI cleanup."
)

// Settings are the choices the Dictation Settings form edits. DeviceID "default" is the system microphone.
type Settings struct {
	DeviceID         string `json:"deviceId"`
	CleanupWithAI    bool   `json:"cleanupWithAi"`
	Dictionary       string `json:"dictionary"`
	CopyToClipboard  bool   `json:"copyToClipboard"`
	UseScreenContext bool   `json:"useScreenContext"`
	ScreenTermLimit  int    `json:"screenTermLimit"`
}

// defaultSettings are the choices a new install starts with.
func defaultSettings() Settings {
	return Settings{DeviceID: defaultDeviceID, ScreenTermLimit: cleanup.DefaultScreenTermLimit}
}

// readSettings returns the saved choices, else the app's legacy choices, else the defaults. A value the storage cannot
// read falls back to the defaults, as 1.x did.
func (e Extension) readSettings(ctx api.Context) Settings {
	settings := defaultSettings()
	if e.legacy.Settings != nil {
		settings = normalizedSettings(*e.legacy.Settings)
	}
	var saved Settings
	if found, err := ctx.Storage().Get(storageSettingsKey, &saved); err == nil && found {
		settings = normalizedSettings(saved)
	}
	return settings
}

// normalizedSettings keeps the values the form can show: a device id, and a screen term limit the form offers.
func normalizedSettings(settings Settings) Settings {
	if settings.DeviceID == "" {
		settings.DeviceID = defaultDeviceID
	}
	settings.ScreenTermLimit = cleanup.NormalizeScreenTermLimit(settings.ScreenTermLimit)
	return settings
}

// aiAvailable reports whether the model can answer for this account. An AI service that fails counts as unavailable.
func aiAvailable(ctx api.Context) bool {
	available, err := ctx.AI().IsAvailable()
	return err == nil && available
}

// openSettings shows the settings form, with the microphones and the AI state read now.
func (e Extension) openSettings(ctx api.Context) (api.Result, error) {
	return pushView(ctx, e.settingsView(ctx)), nil
}

// settingsView is the Dictation Settings form. Each field is a palette row, and the save row stores all of them.
func (e Extension) settingsView(ctx api.Context) api.View {
	settings := e.readSettings(ctx)
	available := aiAvailable(ctx)
	fields := []api.Field{
		deviceField(settings.DeviceID, devicesOf(ctx)),
		cleanupField(settings.CleanupWithAI && available, available),
		dictionaryField(settings.Dictionary),
		checkboxField(fieldClipboard, "Copy transcription to clipboard", settings.CopyToClipboard,
			"Keep the transcription on the clipboard after pasting."),
		checkboxField(fieldScreen, "Use screen text", settings.UseScreenContext,
			"Read words from the screen to fix names and terms. Off unless you turn it on."),
		screenLimitField(settings.ScreenTermLimit),
	}
	return api.FormView(settingsTitle, fields, api.Run(refSaveSettings, settingsSave, nil))
}

// devicesOf lists the microphones the picker offers. The system default is always first.
func devicesOf(ctx api.Context) []api.DictationDevice {
	devices, err := ctx.Dictation().Devices()
	if err != nil || len(devices) == 0 {
		return []api.DictationDevice{{ID: defaultDeviceID, Title: "Default", IsDefault: true}}
	}
	return devices
}

func deviceField(selected string, devices []api.DictationDevice) api.Field {
	field := api.FieldOf(fieldDevice, api.FieldDropdown, "Microphone")
	field.Value = api.TextValue(selected)
	for _, device := range devices {
		id := device.ID
		if device.IsDefault && (id == "" || id == defaultDeviceID) {
			id = defaultDeviceID
		}
		field.Options = append(field.Options, api.Option{Title: device.Title, Value: id})
	}
	return field
}

// cleanupField is the AI cleaning switch. Without an account the switch is shown off and disabled.
func cleanupField(enabled, available bool) api.Field {
	field := api.FieldOf(fieldCleanup, api.FieldCheckbox, "Clean with AI")
	field.Value = api.FlagValue(enabled)
	field.Disabled = !available
	field.Description = "Sign in to Nevermind to use AI cleaning."
	if available {
		field.Description = "Send the transcript to Fast AI to correct punctuation, grammar, and transcription errors before pasting."
	}
	return field
}

func dictionaryField(dictionary string) api.Field {
	field := api.FieldOf(fieldDictionary, api.FieldTextarea, "Custom dictionary")
	field.Value = api.TextValue(dictionary)
	field.Rows = dictionaryRows
	field.Placeholder = dictionaryHint
	field.Description = dictionaryDetails
	return field
}

func checkboxField(id, label string, value bool, description string) api.Field {
	field := api.FieldOf(id, api.FieldCheckbox, label)
	field.Value = api.FlagValue(value)
	field.Description = description
	return field
}

func screenLimitField(limit int) api.Field {
	field := api.FieldOf(fieldScreenLimit, api.FieldDropdown, "Screen term limit")
	field.Value = api.TextValue(strconv.Itoa(limit))
	for _, option := range cleanup.ScreenTermLimits {
		field.Options = append(field.Options, api.Option{
			Title: strconv.Itoa(option) + " terms", Value: strconv.Itoa(option),
		})
	}
	return field
}

// saveSettings stores the form's choices, and returns to the page that opened the form. Cleaning with AI stays off
// when the account cannot use it, as 1.x stored it.
func (e Extension) saveSettings(ctx api.Context, form api.FormValues) (api.Result, error) {
	limit, err := strconv.Atoi(form[fieldScreenLimit].Text)
	if err != nil {
		limit = cleanup.DefaultScreenTermLimit
	}
	settings := normalizedSettings(Settings{
		DeviceID:         form[fieldDevice].Text,
		CleanupWithAI:    form[fieldCleanup].Flag && aiAvailable(ctx),
		Dictionary:       form[fieldDictionary].Text,
		CopyToClipboard:  form[fieldClipboard].Flag,
		UseScreenContext: form[fieldScreen].Flag,
		ScreenTermLimit:  limit,
	})
	if err := ctx.Storage().Set(storageSettingsKey, settings); err != nil {
		return api.Result{}, err
	}
	return api.Result{Navigation: api.NavPop}, nil
}
