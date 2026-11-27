package dictation

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// storageHistoryKey names the transcripts in the built-in's storage. The settings are declared, so the host keeps them.
const storageHistoryKey = "history"

// SettingDevice is the setting whose display text is the microphone's title. The Settings page shows it on its Dictation row.
const SettingDevice = "device"

// The declared settings, by the ids 1.x named them.
const (
	SettingDeviceID   = "deviceId"
	SettingEngine     = "engine"
	SettingCleanup    = "cleanupWithAi"
	SettingDictionary = "dictionary"
	SettingClipboard  = "copyToClipboard"
	SettingScreen     = "useScreenContext"
	defaultDeviceID   = "default"
)

// systemDefaultTitle names the system microphone when the host lists no title for it.
const systemDefaultTitle = "System Default"

// Settings are the dictation choices, read from the declared settings. DeviceID "default" is the system microphone.
type Settings struct {
	DeviceID         string
	CleanupWithAI    bool
	Dictionary       string
	CopyToClipboard  bool
	UseScreenContext bool
	Engine           api.DictationEngine
}

// engineChoices are where speech becomes text: the cloud, or the model on this device.
var engineChoices = []api.SettingChoice{
	{Value: string(api.DictationEngineCloud), Title: "Cloud"},
	{Value: string(api.DictationEngineDevice), Title: "On This Device"},
}

// settingDefinitions declares the dictation settings with the values a new install starts with. The Dictionary is a
// string with one term per line: no declared type holds a list, and the cleanup prompt reads the lines as they are.
func settingDefinitions() []api.SettingDefinition {
	return []api.SettingDefinition{
		{ID: SettingDevice, Title: "Microphone", Type: api.SettingString},
		{ID: SettingDeviceID, Title: "Microphone", Type: api.SettingString, Value: jsonOf(defaultDeviceID),
			Description: `The id of the microphone dictation records from; "default" is the system microphone.`},
		{ID: SettingEngine, Title: "Transcription", Type: api.SettingString, Value: jsonOf(string(api.DictationEngineCloud)),
			Choices: engineChoices, Description: "Where speech becomes text: the cloud, or the free on-device model."},
		{ID: SettingCleanup, Title: "Clean with AI", Type: api.SettingBoolean, Value: jsonOf(false),
			Description: "Send the transcript to Fast AI to correct punctuation, grammar, and transcription errors before pasting."},
		{ID: SettingDictionary, Title: "Custom Dictionary", Type: api.SettingString, Value: jsonOf(""),
			Description: "Preferred terms and spellings for AI cleanup, one per line."},
		{ID: SettingClipboard, Title: "Copy Transcription to Clipboard", Type: api.SettingBoolean, Value: jsonOf(false),
			Description: "Keep the transcription on the clipboard after pasting."},
		{ID: SettingScreen, Title: "Use Screen Text", Type: api.SettingBoolean, Value: jsonOf(false),
			Description: "Read words from the screen to fix names and terms."},
	}
}

func jsonOf(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

// readSettings reads each declared setting. A setting the host cannot read keeps its default.
func readSettings(ctx api.Context) Settings {
	settings := Settings{DeviceID: defaultDeviceID}
	var engine string
	store := ctx.Settings()
	_ = store.Get(SettingDeviceID, &settings.DeviceID)
	_ = store.Get(SettingEngine, &engine)
	_ = store.Get(SettingCleanup, &settings.CleanupWithAI)
	_ = store.Get(SettingDictionary, &settings.Dictionary)
	_ = store.Get(SettingClipboard, &settings.CopyToClipboard)
	_ = store.Get(SettingScreen, &settings.UseScreenContext)
	settings.Engine = engineOrCloud(api.DictationEngine(engine))
	if settings.DeviceID == "" {
		settings.DeviceID = defaultDeviceID
	}
	return settings
}

// aiAvailable reports whether the model can answer for this account. An AI service that fails counts as unavailable.
func aiAvailable(ctx api.Context) bool {
	available, err := ctx.AI().IsAvailable()
	return err == nil && available
}

// devicesOf lists the microphones the host knows. A host that cannot list them lists none.
func devicesOf(ctx api.Context) []api.DictationDevice {
	devices, err := ctx.Dictation().Devices()
	if err != nil {
		return nil
	}
	return devices
}

// publishDeviceTitle publishes the title of the microphone the settings choose, for the Settings page's Dictation row.
// A host that cannot list microphones leaves the row with its own text, so the error is not shown.
func publishDeviceTitle(ctx api.Context, deviceID string) {
	_ = ctx.Settings().PublishDisplay(SettingDevice, displayTitleOf(devicesOf(ctx), deviceID))
}

// displayTitleOf is the microphone's title for the Settings page: its listed title, else the system microphone's name
// for the default id, else the id itself.
func displayTitleOf(devices []api.DictationDevice, id string) string {
	if title := deviceTitle(devices, id); title != "" {
		return title
	}
	if isDefaultDevice(id) {
		return systemDefaultTitle
	}
	return id
}
