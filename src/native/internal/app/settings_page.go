package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// settingsPage is the settings page the root's Settings command and the menu bar open. The Settings built-in declares
// its rows, and the palette draws them.
func (p *App) settingsPage() *palette.View {
	return p.builtInOpenPage(settings.ID, unavailableSettingsPage())
}

func unavailableSettingsPage() *palette.View {
	return &palette.View{Title: "Settings", Placeholder: "Search Settings", Search: func(string) []search.Item { return nil }}
}
