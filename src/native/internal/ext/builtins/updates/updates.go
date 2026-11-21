// Package updates is the Updates built-in: the app's update state, a check for a newer version, the restart into a
// downloaded one and the channel switch. The page reads the state through the updates service, and its rows run
// ActionUpdate actions the host performs. The root's update rows and the menu bar stay the host's.
//
// Layer: ext. The package imports api and the standard library.
package updates

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ID is the built-in's identity.
const ID = "nevermind.updates"

// Refs of the page's actions: open builds the page and refresh re-reads the state while the page is visible.
const (
	openAction    = "open"
	refreshAction = "refresh"
)

// refreshIntervalMs is how often the open page re-reads the updater, so a check or a download shows as it moves.
const refreshIntervalMs = 1000

// viewTitle names the page, as the native page named it.
const viewTitle = "Updates"

// Extension is the Updates built-in. It keeps no state: the host's updater reports every state.
type Extension struct{}

// New returns the Updates built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. Its state and its actions are the app's updates, which the updates capability covers.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: viewTitle},
		Capabilities: []api.Capability{api.CapabilityUpdates},
	}
}

// RunAction opens the page, or re-reads the state for the open page.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case openAction:
		view := updatesView(ctx)
		return api.Result{View: &view}, nil
	case refreshAction:
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: updateRows(ctx.Updates().State())}}, nil
	}
	return api.Result{}, fmt.Errorf("the Updates built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// updatesView is the page as it stands now, refreshed while it is open.
func updatesView(ctx api.Context) api.View {
	view := api.ListView(viewTitle, updateRows(ctx.Updates().State())...)
	view.Placeholder = "Search updates"
	view.Refresh = &api.Refresh{Ref: &api.ActionRef{ID: refreshAction}, IntervalMs: refreshIntervalMs, Mode: api.PatchReplace}
	return view
}
