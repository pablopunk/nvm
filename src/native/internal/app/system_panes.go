package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// systemSettingsPanes lists the settings panes for the extension system service.
func (p *App) systemSettingsPanes() []api.SettingsPane {
	var out []api.SettingsPane
	for _, pane := range p.systemSvc.SettingsPanes() {
		out = append(out, api.SettingsPane{ID: pane.ID, Title: pane.Title})
	}
	return out
}

// openSystemPaneByID opens the settings pane an extension names, if the OS has it.
func (p *App) openSystemPaneByID(id string) {
	for _, pane := range p.systemSvc.SettingsPanes() {
		if pane.ID == id {
			openSettingsPane(p, pane)
			return
		}
	}
}

// openSettingsPane opens one settings pane off the UI thread.
func openSettingsPane(p *App, pane systemfeat.Pane) {
	safego.Go("system.pane", func(context.Context) {
		if err := p.systemSvc.OpenPane(pane, ""); err != nil {
			p.toastFromWorker(describeSystemFailure(pane.Title, err))
		}
	})
}
