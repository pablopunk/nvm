// Package diagnostics is the Diagnostics built-in: the errors the host recorded, each with a copyable support reference and
// its safe text, and the Report a Problem command that files a reference without screen content. The root's Show Recent
// Errors and Report a Problem commands open its page and run its report action.
//
// Layer: ext. The package imports api and the standard library.
package diagnostics

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ID is the built-in's identity.
const ID = "nevermind.diagnostics"

// Refs of the page's actions. Open builds the page; Report files a problem report and shows the page with its toast.
const (
	OpenAction   = "open"
	ReportAction = "report"
)

// viewTitle, placeholder, hint and emptyTitle name the page, as the native page named it.
const (
	viewTitle   = "Recent Errors"
	placeholder = "Search recent errors"
	hint        = "Technical details only"
	emptyTitle  = "No recent errors recorded"
)

// Extension is the Diagnostics built-in. It keeps no state: the host keeps the records.
type Extension struct{}

// New returns the Diagnostics built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. Reading the records and filing a report need no declaration.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion: "1.0",
		Identity:   api.Identity{ID: ID, Title: "Diagnostics"},
	}
}

// RunAction opens the page, or files a problem report and shows the page with its reference.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case OpenAction:
		view := recentErrorsView(ctx)
		return api.Result{View: &view}, nil
	case ReportAction:
		return reportResult(ctx), nil
	}
	return api.Result{}, fmt.Errorf("the Diagnostics built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// reportResult files a problem report. The page lists the recent errors, where the new reference appears, and a toast
// names it. A host that cannot record a report says so instead.
func reportResult(ctx api.Context) api.Result {
	ref := ctx.Diagnostics().ReportProblem()
	if ref == "" {
		return api.Result{Toast: &api.Toast{Message: "The problem report could not be recorded", Tone: "error"}}
	}
	view := recentErrorsView(ctx)
	return api.Result{View: &view, Toast: &api.Toast{Message: "Problem report " + ref + " recorded"}}
}

// recentErrorsView is the page as it stands now: one row per recorded error, newest first.
func recentErrorsView(ctx api.Context) api.View {
	view := api.ListView(viewTitle, recordRows(ctx)...)
	view.Placeholder = placeholder
	view.Hint = hint
	view.EmptyView = &api.EmptyState{Title: emptyTitle}
	return view
}
