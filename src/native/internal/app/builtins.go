package app

import (
	"context"
	"slices"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aicommands"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	appsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/apps"
	backgroundtasksbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/backgroundtasks"
	calculatorbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/calculator"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/classifyimage"
	diagnosticsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/diagnostics"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/gettingstarted"
	permissionsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/permissions"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/quicklinks"
	settingsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/settings"
	shortcutsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/shortcuts"
	snippetsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/syspanes"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	timersbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/timers"
	updatesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/updates"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/websearch"
	"github.com/pablopunk/nvm/native/internal/features/calc"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// builtInInputs are the values the settings file held before the built-ins kept their own state. Each built-in reads
// its value until its own storage or setting holds one. Rates is the calculator's exchange-rate cache and Zone the zone
// its calendar answers use.
type builtInInputs struct {
	Rates        calc.RateCache
	Zone         *time.Location
	Quicklinks   []quicklinks.Link
	Snippets     []snippetsbuiltin.Snippet
	GuideDone    bool
	SearchEngine string
}

// builtInExtensions are the extensions compiled into the app. They run in the host, are always on, and answer the
// root and the palette through the same manager as folder extensions.
func builtInExtensions(in builtInInputs) []api.Extension {
	return []api.Extension{
		syspanes.Extension{}, systemcommands.Extension{}, appcommands.Extension{}, placesbuiltin.Extension{},
		calculatorbuiltin.New(in.Rates, in.Zone), classifyimage.Extension{}, aicommands.New(),
		quicklinks.New(in.Quicklinks), snippetsbuiltin.New(in.Snippets), gettingstarted.New(in.GuideDone), websearch.New(in.SearchEngine),
		timersbuiltin.New(),
		appsbuiltin.New(), settingsbuiltin.New(), permissionsbuiltin.New(), updatesbuiltin.New(), diagnosticsbuiltin.New(), backgroundtasksbuiltin.New(), shortcutsbuiltin.New(),
	}
}

// commandPlacedBuiltIns are the built-ins whose rows the command list places among the native commands (see
// commandRows). The extensions source leaves them out, so each row is listed once, at its place.
var commandPlacedBuiltIns = []string{systemcommands.ID, appcommands.ID, placesbuiltin.ID, classifyimage.ID}

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
	return searchRowsFor(p, m, q, result, nil)
}
