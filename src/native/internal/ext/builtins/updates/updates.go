// Package updates is the Updates built-in: the app's update state, a check for a newer version, the restart into a
// downloaded one and the channel choice, and the root row that shows an update as it downloads. The page and the root
// row read the state through the updates service, and their rows run ActionUpdate actions the host performs. The host
// redraws both whenever its updater changes, so a download's progress moves on screen. The menu bar stays the host's.
//
// Layer: ext. The package imports api, search (for the root section name) and the standard library.
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

// refreshIntervalMs is how often the open page re-reads the updater by itself, so "Checked 5 minutes ago" stays true.
// Every change of the updater redraws the page at once; this interval only ages the time of the last check.
const refreshIntervalMs = 30_000

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
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: updateRows(ctx.Updates().State(), ctx.Clock().Now())}}, nil
	}
	return api.Result{}, fmt.Errorf("the Updates built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// updatesView is the page as it stands now, with the status row selected and refreshed while it is open.
func updatesView(ctx api.Context) api.View {
	view := api.ListView(viewTitle, updateRows(ctx.Updates().State(), ctx.Clock().Now())...)
	view.Placeholder = "Search updates"
	view.SelectedItemID = rowStatus
	view.Refresh = &api.Refresh{Ref: &api.ActionRef{ID: refreshAction}, IntervalMs: refreshIntervalMs, Mode: api.PatchReplace}
	return view
}
