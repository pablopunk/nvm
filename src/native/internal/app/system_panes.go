package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// systemSettingsPanes lists the settings panes for the extension system service.
func (p *App) systemSettingsPanes() []api.SettingsPane {
	var out []api.SettingsPane
	for _, pane := range p.systemSvc.SettingsPanes() {
		out = append(out, api.SettingsPane{ID: pane.ID, Title: pane.Title})
	}
	return out
}
