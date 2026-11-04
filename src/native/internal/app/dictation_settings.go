package app

import (
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// dictationSettingsFile holds the dictation choices in the user's store.
const dictationSettingsFile = "dictation.json"

// dictationRecordingsFolder holds the audio of dictations whose text is not in yet, inside the data folder.
const dictationRecordingsFolder = "dictation-recordings"

// dictationSettings are the choices the Dictation Settings view edits. DeviceID "" is the system default microphone.
type dictationSettings struct {
	DeviceID        string `json:"deviceId"`
	CopyToClipboard bool   `json:"copyToClipboard"`
	CleanupWithAI   bool   `json:"cleanupWithAi"`
	Dictionary      string `json:"dictionary"`
	// UseScreenContext reads the words on screen for the cleanup prompt; ScreenTermLimit is how many of them it names.
	UseScreenContext bool `json:"useScreenContext"`
	ScreenTermLimit  int  `json:"screenTermLimit"`
}

// dictationPrefs reads and changes the settings. The store keeps them on disk; without a writable store they last
// for the process.
type dictationPrefs interface {
	Load() (dictationSettings, error)
	Update(change func(*dictationSettings)) error
}

// memoryPrefs keeps the settings in memory.
type memoryPrefs struct {
	value dictationSettings
}

func (m *memoryPrefs) Load() (dictationSettings, error) {
	return m.value, nil
}

func (m *memoryPrefs) Update(change func(*dictationSettings)) error {
	change(&m.value)
	return nil
}

// dictationPrefsStore returns the settings store, created on first use from the user's store.
func (p *App) dictationPrefsStore() dictationPrefs {
	if p.dictationPref == nil {
		if p.userStore == nil || p.userReadOnly {
			p.dictationPref = &memoryPrefs{}
		} else {
			p.dictationPref = store.Doc(p.userStore, dictationSettingsFile, 1, func() dictationSettings { return dictationSettings{} })
		}
	}
	return p.dictationPref
}

// dictationSettings reads the current choices; a settings file that cannot be read falls back to the defaults.
func (p *App) dictationSettings() dictationSettings {
	value, err := p.dictationPrefsStore().Load()
	if err != nil {
		return dictationSettings{}
	}
	return value
}

// setDictationSettings changes the choices and saves them.
func (p *App) setDictationSettings(change func(*dictationSettings)) {
	_ = p.dictationPrefsStore().Update(change)
}

// deviceTitle names the chosen microphone for the settings row.
func deviceTitle(devices []platform.MicrophoneDevice, id string) string {
	if id == "" {
		return "System Default"
	}
	for _, device := range devices {
		if device.ID == id {
			return device.Title
		}
	}
	return id
}
