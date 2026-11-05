// Package apps is the built-in that lists the installed apps at the root and finds them by name or alias. Enter opens
// an app, and each row's menu holds Uninstall and the shortcut and alias actions. The Force Quit Apps and Quit All Apps
// commands quit the running ones, and the uninstall review moves an app and its data to the Trash.
//
// Layer: ext. The package imports api, search and the standard library.
package apps

import (
	"cmp"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity.
const ID = "nevermind.apps"

// Refs of the actions the rows, the commands and the uninstall review run. The args carry the app path, its name or the
// review token and item ID the action acts on.
const (
	refOpen             = "open"
	refUninstall        = "uninstall"
	refForceQuit        = "force-quit"
	refQuitAll          = "quit-all"
	refReview           = "uninstall-review"
	refToggle           = "uninstall-toggle"
	refTrash            = "uninstall-trash"
	argPath             = "path"
	argName             = "name"
	argToken            = "token"
	argID               = "id"
	aliasPrefix         = "app:"
	runningStatus       = "Running"
	uninstallCapability = "apps.uninstall"
)

// scoreRootApplication is the 1.x score of an app at the empty root, and of its match in a search before the alias.
const scoreRootApplication = 55

// Extension is the applications built-in. It keeps the uninstall reviews it shows, so a toggle finds the selection the
// review holds.
type Extension struct {
	reviews *reviewBook
}

// New returns the applications built-in.
func New() Extension {
	return Extension{reviews: newReviewBook()}
}

// Manifest identifies the built-in. Its rows are listed as the native app rows were, under the Applications section.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: "Applications"},
		Capabilities: []api.Capability{api.CapabilityDesktopApps, api.CapabilityDesktopFiles, api.CapabilityShortcuts, api.CapabilitySystem},
	}
}

// RootItems lists every app, newest first by when the host first saw it, then by name, as 1.x lists them.
func (Extension) RootItems(ctx api.Context) ([]api.Item, error) {
	list, err := ctx.Desktop().Apps().List()
	if err != nil {
		return nil, err
	}
	sorted := slices.Clone(list)
	slices.SortStableFunc(sorted, func(a, b api.App) int {
		return cmp.Or(cmp.Compare(b.FirstSeenMs, a.FirstSeenMs), strings.Compare(a.Name, b.Name))
	})
	return appItems(ctx, sorted, func(api.App) float64 { return scoreRootApplication }), nil
}

// SearchItems finds the apps whose name matches the query, or whose alias does. An alias scores the row when the name
// does not, as 1.x's aliases do.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	list, err := ctx.Desktop().Apps().List()
	if err != nil {
		return nil, err
	}
	lq := strings.ToLower(query)
	aliases := ctx.Shortcuts().Aliases(aliasPrefix)
	var matched []api.App
	var scores []float64
	for _, app := range list {
		score := search.Score(app.Name, lq)
		if score == 0 {
			score = aliasScore(aliases[aliasPrefix+app.Path], lq)
		}
		if score > 0 {
			matched = append(matched, app)
			scores = append(scores, float64(score))
		}
	}
	return appItemsScored(ctx, matched, scores), nil
}

// aliasScore is how well the best of an app's aliases answers the query, 0 when none does.
func aliasScore(aliases []string, query string) int {
	best := 0
	for _, alias := range aliases {
		best = max(best, search.Score(alias, query))
	}
	return best
}

// appItems builds the rows of apps with the score score gives each.
func appItems(ctx api.Context, list []api.App, score func(api.App) float64) []api.Item {
	scores := make([]float64, len(list))
	for i, app := range list {
		scores[i] = score(app)
	}
	return appItemsScored(ctx, list, scores)
}

// appItemsScored builds one row per app, with its score. Running state is read once for the whole list.
func appItemsScored(ctx api.Context, list []api.App, scores []float64) []api.Item {
	running := runningAmong(ctx, list)
	uninstallable := ctx.System().HasCapability(uninstallCapability)
	items := make([]api.Item, 0, len(list))
	for i, app := range list {
		items = append(items, appItem(app, running[app.Path], uninstallable, scores[i]))
	}
	return items
}

// appItem is the row of one app: its name, Running while it runs, its icon, and the actions of its menu.
func appItem(app api.App, running, uninstallable bool, score float64) api.Item {
	open := api.Run(refOpen, "Open "+app.Name, map[string]string{argPath: app.Path})
	open.DismissAfterRun = true
	item := api.Item{
		Title: app.Name, Section: search.KindApplications, LearnKey: "app:" + app.ID,
		CustomizeAs: aliasPrefix + app.Path,
		File:        &api.FileRef{Path: app.Path, Name: app.Name, Kind: "file"},
		Score:       score, LastUsedMs: app.FirstSeenMs,
		Primary: &open,
	}
	if running {
		item.Subtitle = runningStatus
	}
	if uninstallable {
		uninstall := api.Destructive(api.Run(refUninstall, "Uninstall "+app.Name+"…", map[string]string{argPath: app.Path, argName: app.Name}))
		item.Actions = []api.Action{uninstall}
	}
	return item
}

// runningAmong reports which of the apps has a live process. The host answers from the state it keeps, so the rows
// never wait for a scan; the running set changing refreshes the rows.
func runningAmong(ctx api.Context, list []api.App) map[string]bool {
	paths := make([]string, 0, len(list))
	for _, app := range list {
		paths = append(paths, app.Path)
	}
	running := map[string]bool{}
	for chunk := range slices.Chunk(paths, api.MaxRunningPaths) {
		answered, err := ctx.Desktop().Apps().Running(chunk)
		if err != nil {
			return running
		}
		for _, path := range answered {
			running[path] = true
		}
	}
	return running
}

// RunAction runs the open, uninstall, force quit and quit-all actions of the rows and commands, and the review's steps.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	args := in.Ref.Args
	switch in.Ref.ID {
	case refOpen:
		return api.Result{}, ctx.Desktop().Apps().Launch(args[argPath])
	case refUninstall:
		return e.startUninstall(ctx, args[argPath], args[argName])
	case refReview, refToggle, refTrash:
		return e.runReview(ctx, in.Ref.ID, args)
	case refForceQuit:
		return forceQuit(ctx, args[argPath], args[argName])
	case ForceQuitCommandID:
		return forceQuitCommand(ctx)
	case QuitAllCommandID:
		return quitAllCommand()
	case refQuitAll:
		return quitAll(ctx)
	}
	return api.Result{}, unknownAction(in.Ref.ID)
}
