// Package shortcuts is the Keyboard Shortcuts built-in: the global shortcuts the user has bound, each with its label, and
// the actions that change one or remove the shortcut the user chose. The root's Keyboard Shortcuts command opens its page.
//
// Layer: ext. The package imports api and the standard library.
package shortcuts

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ID is the built-in's identity.
const ID = "nevermind.shortcuts"

// openAction is the action the root's Keyboard Shortcuts command runs to get the page.
const openAction = "open"

// title and placeholder name the page, as the native page named it.
const (
	viewTitle   = "Keyboard Shortcuts"
	placeholder = "Search Keyboard Shortcuts"
	hint        = "↵ Change"
	emptyTitle  = "No shortcuts found."
)

// Extension is the Keyboard Shortcuts built-in. It keeps no state: the host's shortcut table holds every binding.
type Extension struct{}

// New returns the Keyboard Shortcuts built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. Reading the table and changing a shortcut are the shortcuts capability.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: viewTitle},
		Capabilities: []api.Capability{api.CapabilityShortcuts},
	}
}

// RunAction opens the page.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID == openAction {
		view := shortcutsView(ctx)
		return api.Result{View: &view}, nil
	}
	return api.Result{}, fmt.Errorf("the Keyboard Shortcuts built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// shortcutsView is the page: one row per bound shortcut, with its label as the accessory.
func shortcutsView(ctx api.Context) api.View {
	records := ctx.Shortcuts().List()
	rows := make([]api.Item, 0, len(records))
	for _, record := range records {
		rows = append(rows, shortcutRow(record))
	}
	view := api.ListView(viewTitle, rows...)
	view.Placeholder = placeholder
	view.Hint = hint
	view.EmptyView = &api.EmptyState{Title: emptyTitle}
	return view
}
