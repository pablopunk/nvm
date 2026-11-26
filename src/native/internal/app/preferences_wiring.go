package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// extensionPreferenceEditor reads and changes every extension's declared settings for extensions and conversation
// programs. Saves run on the UI thread, which owns the Settings page's values; reads never wait for it.
func (p *App) extensionPreferenceEditor() services.PreferenceEditor {
	return manager.NewPreferenceEditor(p.preferenceSource, p.onUI, p.preferenceChanged, settings.RecordedElsewhere())
}

// preferenceSource is the extension manager, or nil before the extensions start. The nil is an untyped nil interface.
func (p *App) preferenceSource() manager.PreferenceSource {
	if m := p.extensionManager(); m != nil {
		return m
	}
	return nil
}

// preferenceChanged drops the root rows of the extension whose setting changed and runs the visible Settings page or
// page of that extension again, so they show the new value. It runs on the UI thread.
func (p *App) preferenceChanged(extensionID string) {
	p.forgetRootRowsOf(extensionID)
	if p.Palette == nil {
		return
	}
	p.RefreshTop(func(v *palette.View) bool {
		return v.ExtensionID == extensionID || v.ExtensionID == settings.ID
	})
}
