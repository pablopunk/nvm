// Package backgroundtasks is the Background Tasks built-in: every background job the host schedules, with its status,
// triggers and recent runs, and the actions that run one now, turn it on or off, and clear its error. The root's
// Background Tasks command opens its page.
//
// Layer: ext. The package imports api and the standard library.
package backgroundtasks

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ID is the built-in's identity.
const ID = "nevermind.background-tasks"

// Refs of the page's actions. Open builds the page and refresh re-reads it while it is visible; run, toggle and clear
// change one job and show the page again.
const (
	openAction    = "open"
	refreshAction = "refresh"
	runAction     = "run"
	toggleAction  = "toggle"
	clearAction   = "clear-error"
	argID         = "id"
	argEnabled    = "enabled"
)

// refreshIntervalMs is how often the open page re-reads the jobs, so a run that finishes shows as it ends.
const refreshIntervalMs = 1000

// viewTitle and placeholder name the page, as the native page named it.
const (
	viewTitle   = "Background Tasks"
	placeholder = "Search background tasks"
	emptyTitle  = "No background tasks registered."
)

// Extension is the Background Tasks built-in. It keeps no state: the host's scheduler holds every job.
type Extension struct{}

// New returns the Background Tasks built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. Changing a job's state needs the jobs declaration, which the built-in declares for review.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: viewTitle},
		Capabilities: []api.Capability{api.CapabilityJobs},
	}
}

// RunAction opens the page, re-reads it, or changes one job.
func (Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case openAction:
		view := jobsView(ctx)
		return api.Result{View: &view}, nil
	case refreshAction:
		return api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: jobRows(ctx)}}, nil
	case runAction:
		return runResult(ctx, in.Ref.Args[argID]), nil
	case toggleAction:
		return toggleResult(ctx, in.Ref.Args[argID], in.Ref.Args[argEnabled] == "true"), nil
	case clearAction:
		return clearResult(ctx, in.Ref.Args[argID]), nil
	}
	return api.Result{}, fmt.Errorf("the Background Tasks built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// jobsView is the page as it stands now, refreshed while it is open.
func jobsView(ctx api.Context) api.View {
	view := api.ListView(viewTitle, jobRows(ctx)...)
	view.Placeholder = placeholder
	view.EmptyView = &api.EmptyState{Title: emptyTitle}
	view.Refresh = &api.Refresh{Ref: &api.ActionRef{ID: refreshAction}, IntervalMs: refreshIntervalMs, Mode: api.PatchReplace}
	return view
}

// showPage is the result that replaces the page with its jobs as they stand now, and says what happened in a toast.
func showPage(ctx api.Context, message string) api.Result {
	view := jobsView(ctx)
	return api.Result{View: &view, Navigation: api.NavReplace, Toast: &api.Toast{Message: message}}
}

// runResult runs a job now. A run that fails shows the reason as a page, as the native page does.
func runResult(ctx api.Context, id string) api.Result {
	job, ok := findJob(ctx, id)
	if !ok {
		return unknownJob(id)
	}
	if err := ctx.Jobs().Run(id); err != nil {
		view := api.PreviewView(job.Title+" Failed", "# "+job.Title+" Failed\n\n"+err.Error())
		return api.Result{View: &view}
	}
	return showPage(ctx, "Ran "+job.Title)
}

// toggleResult turns a job on or off. A refusal is an error toast.
func toggleResult(ctx api.Context, id string, enabled bool) api.Result {
	job, ok := findJob(ctx, id)
	if !ok {
		return unknownJob(id)
	}
	if err := ctx.Jobs().SetEnabled(id, enabled); err != nil {
		return api.Result{Toast: &api.Toast{Message: err.Error(), Tone: "error"}}
	}
	if enabled {
		return showPage(ctx, "Enabled "+job.Title)
	}
	return showPage(ctx, "Disabled "+job.Title)
}

// clearResult forgets the last error of a job. A refusal is an error toast.
func clearResult(ctx api.Context, id string) api.Result {
	job, ok := findJob(ctx, id)
	if !ok {
		return unknownJob(id)
	}
	if err := ctx.Jobs().ClearError(id); err != nil {
		return api.Result{Toast: &api.Toast{Message: err.Error(), Tone: "error"}}
	}
	return showPage(ctx, "Cleared "+job.Title)
}

// findJob returns the job with the id, as the host lists it now.
func findJob(ctx api.Context, id string) (api.Job, bool) {
	for _, job := range ctx.Jobs().List() {
		if job.ID == id {
			return job, true
		}
	}
	return api.Job{}, false
}

// unknownJob refuses an action on a job the host does not hold.
func unknownJob(id string) api.Result {
	return api.Result{Toast: &api.Toast{Message: "There is no background job " + id, Tone: "error"}}
}
