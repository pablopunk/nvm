package app

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	"github.com/pablopunk/nvm/native/internal/search"
)

// settingRowAccessory is the host's text for the rows whose value only the host knows: the shortcut's status, the
// dictation device, the updates status, the account and the config folder path. It answers for the Settings built-in
// only.
func (p *App) settingRowAccessory(extID, rowID string) (string, string, bool) {
	if extID != settings.ID {
		return "", "", false
	}
	switch rowID {
	case settings.SettingPaletteShortcut:
		return p.paletteSettingAccessory(), "", true
	case settings.RowDictation:
		return deviceTitle(p.dictationFlowFor().Devices(), p.dictationSettings().DeviceID), "", true
	case settings.RowUpdates:
		return updateAccessory(p.updateSnapshot().State), "", true
	case settings.RowAccount:
		return p.accountSummary(), "", true
	case settings.RowConfigFolder:
		return "", p.displayPath(sharedDir()), true
	}
	return "", "", false
}

// settingsHostOperation runs one host page or folder action of the Settings rows.
func (p *App) settingsHostOperation(h search.Host, extID, op string) {
	if extID != settings.ID {
		return
	}
	switch op {
	case "config-folder":
		h.Hide()
		_ = mygo.Shell.OpenPath(sharedDir())
	case "dictation-settings":
		h.Push(p.dictationSettingsView())
	case "updates":
		h.Push(p.updatesPage())
	case "account-page":
		p.openAccountPage(h)
	}
}

// settingRecorder pushes the host's recorder for a shortcut row. Key capture stays with the host.
func (p *App) settingRecorder(h search.Host, extID, key string) {
	if extID != settings.ID {
		return
	}
	switch key {
	case settings.SettingPaletteShortcut:
		h.Push(p.paletteRecorderView())
	case settings.SettingHyperKey:
		h.Push(p.hyperRecorderView())
	}
}
