package app

import (
	"context"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/syspanes"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// builtInExtensions are the extensions compiled into the app. They run in the host, are always on, and answer the
// root and the palette through the same manager as folder extensions.
func builtInExtensions() []api.Extension {
	return []api.Extension{syspanes.Extension{}, systemcommands.Extension{}, appcommands.Extension{}, placesbuiltin.Extension{}}
}

// commandPlacedBuiltIns are the built-ins whose rows the command list places among the native commands (see
// commandRows). The extensions source leaves them out, so each row is listed once, at its place.
var commandPlacedBuiltIns = []string{systemcommands.ID, appcommands.ID, placesbuiltin.ID}

// isCommandPlacedBuiltIn reports whether the commands source, not the extensions source, lists a built-in's rows.
func isCommandPlacedBuiltIn(id string) bool {
	return slices.Contains(commandPlacedBuiltIns, id)
}

// systemCapability reports whether the app's port set supports a capability, by its extension name.
func (p *App) systemCapability(name string) bool {
	return p.ports.Capabilities.Supported(platform.Capability(name))
}

// builtInRows are the rows of one command-placed built-in for q, at the place of its command list placeholder. The
// root's empty query has none.
func builtInRows(p *App, id, q string) []search.Item {
	m := p.extensionManager()
	if m == nil || q == "" {
		return nil
	}
	result, ok := m.SearchBuiltIn(context.Background(), id, q)
	if !ok {
		return nil
	}
	return searchRowsFor(p, m, q, result)
}
