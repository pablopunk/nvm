package fixtures

import "github.com/pablopunk/nvm/native/internal/ext/api"

// ExtensionWindowFixture is a companion window an extension opens or restores: the view its page shows and the
// options it opens with.
type ExtensionWindowFixture struct {
	Name    string
	View    api.View
	Options api.WindowOptions
}

// ExtensionWindowFixtures returns the windows the window tests and dev-only fixtures use: a floating window that
// remembers its frame and survives a relaunch, and a plain window with the default options.
func ExtensionWindowFixtures() []ExtensionWindowFixture {
	alwaysOnTop := true
	return []ExtensionWindowFixture{
		{
			Name: "floating-ticker",
			View: api.View{ID: "ticker-floating", Kind: api.ViewList, Title: "Ticker", Subtitle: "Live quotes",
				Items: []api.Item{{ID: "ticker-row", Title: "ACME 142.10", Subtitle: "Up 1.2%"}}},
			Options: api.WindowOptions{
				RestoreKey: "ticker.floating", Persistent: true, RemembersFrame: true, AlwaysOnTop: &alwaysOnTop,
				Size: api.SizeLarge, TitleBar: "hidden",
			},
		},
		{
			Name: "plain-ticker",
			View: api.View{ID: "ticker-plain", Kind: api.ViewList, Title: "Plain",
				Items: []api.Item{{ID: "plain-row", Title: "Plain row"}}},
		},
	}
}
