package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/syspanes"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// builtInExtensions are the extensions compiled into the app. They run in the host, are always on, and answer the
// root and the palette through the same manager as folder extensions.
func builtInExtensions() []api.Extension {
	return []api.Extension{syspanes.Extension{}, systemcommands.Extension{}}
}

// builtInCommandSlot is how many built-in system commands the root lists at their place in the tie order, the number
// the native system commands took when they were in the command list.
func (p *App) builtInCommandSlot() int {
	return systemcommands.SupportedCount(p.systemCapability)
}

// systemCapability reports whether the app's port set supports a capability, by its extension name.
func (p *App) systemCapability(name string) bool {
	return p.ports.Capabilities.Supported(platform.Capability(name))
}

// builtInCommandRows are the rows of the built-in system commands for q, in the place of the native system commands
// (see commandRows). The root's empty query has none.
func builtInCommandRows(p *App, q string) []search.Item {
	m := p.extensionManager()
	if m == nil || q == "" {
		return nil
	}
	result, ok := m.SearchBuiltIn(context.Background(), systemcommands.ID, q)
	if !ok {
		return nil
	}
	return searchRowsFor(p, m, q, result)
}
