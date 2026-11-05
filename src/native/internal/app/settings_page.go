package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

// settingsPageTimeout bounds the call that builds the settings page. The built-in answers in process.
const settingsPageTimeout = 5 * time.Second

// settingsPage is the settings page the root's Settings command and the menu bar open. The Settings built-in declares
// its rows, and the palette draws them.
func (p *App) settingsPage() *palette.View {
	m := p.extensionManager()
	if m == nil {
		return unavailableSettingsPage()
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsPageTimeout)
	defer cancel()
	result, failure, err := m.Run(ctx, settings.ID, api.Invocation{Ref: api.ActionRef{ID: "open"}})
	if err != nil || failure != nil || result.View == nil {
		return unavailableSettingsPage()
	}
	return extview.NewSession(p.extensionViewDeps(m), settings.ID, *result.View).Palette()
}

func unavailableSettingsPage() *palette.View {
	return &palette.View{Title: "Settings", Placeholder: "Search Settings", Search: func(string) []search.Item { return nil }}
}
