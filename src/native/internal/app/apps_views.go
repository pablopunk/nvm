package app

import (
	"context"
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// The apps views: Force Quit Apps, Quit All Apps, and the uninstall review and its result. The first two read live
// state and answer on the UI thread; the review's discovery reads the file system, so it runs as a provider, off the
// UI thread, and its selection is shared with the UI thread under a lock.

// forceQuitView lists the running apps, each with Quit and Force Quit.
func (p *App) forceQuitView() *palette.View {
	return &palette.View{
		Title: "Force Quit Apps", Placeholder: "Search running apps",
		EmptyTitle: "No running apps", EmptySub: "No running applications to force quit.",
		Search: func(q string) []search.Item {
			list := p.appsvc.Index.Apps()
			byPath := appsByPath(list)
			var items []search.Item
			for _, row := range apps.ForceQuitRows(list, p.appsvc.Running.RunningAmong(list)) {
				if q != "" && search.Score(row.Title, strings.ToLower(q)) == 0 {
					continue
				}
				items = append(items, p.appItem(byPath[row.Path], row, 0))
			}
			return items
		},
	}
}

// quitAllConfirm asks once before every visible app is asked to quit, as 1.x does. The apps are read when the user
// confirms, so the quit closes what is running at that moment.
func (p *App) quitAllConfirm() *palette.View {
	return palette.Confirm("Quit All Apps", "This will close all visible applications. Unsaved changes may be lost.", func() {
		p.quitApps(p.runningAppsToQuitAll(), false)
	})
}

// runningAppsToQuitAll lists the running apps Quit All Apps closes: every running app but Nevermind and the exceptions.
func (p *App) runningAppsToQuitAll() []apps.App {
	list := p.appsvc.Index.Apps()
	quit, _ := apps.QuitAllPlan(list, p.appsvc.Running.RunningAmong(list), apps.App{})
	return quit
}

func checkLabel(on bool) string {
	if on {
		return "Selected"
	}
	return "Optional"
}

// uninstallView is the review of one app's uninstall: the items to move to Trash with a toggle each, a confirmation,
// and the result. Discovery runs once, off the UI thread, when the view first loads. The selection lives in this
// closure, guarded by mu, because the rows are built off the UI thread and toggled on it.
func (p *App) uninstallView(a apps.App) *palette.View {
	var (
		mu        sync.Mutex
		once      sync.Once
		discovery apps.Discovery
		selected  = map[string]bool{}
	)
	discover := func() apps.Discovery {
		once.Do(func() {
			discovery = p.appsvc.Uninstaller.Discover(a.Path)
			mu.Lock()
			defer mu.Unlock()
			for _, c := range discovery.Snapshot.Candidates {
				selected[c.ID] = c.SelectedByDefault
			}
		})
		return discovery
	}
	toggle := func(id string) {
		mu.Lock()
		defer mu.Unlock()
		selected[id] = !selected[id]
	}
	rows := func(_ context.Context, q string) []search.Item {
		d := discover()
		if !d.Ready {
			return []search.Item{{Title: "Uninstall unavailable", Subtitle: d.Message, Icon: "x", Tone: "red"}}
		}
		mu.Lock()
		current := maps.Clone(selected)
		mu.Unlock()
		return p.reviewItems(d, current, q, toggle)
	}
	return &palette.View{
		Title: "Uninstall " + a.Name, Placeholder: "Filter items to move to Trash",
		Providers: func() []search.Provider {
			return []search.Provider{search.NewRowsProvider("uninstall-review", rows)}
		},
	}
}

// reviewItems lists the review: the confirm row, then one toggle per candidate, then the discovery's notes. current is
// the selection as the rows are built; toggle flips one candidate and the palette searches again.
func (p *App) reviewItems(d apps.Discovery, current map[string]bool, q string, toggle func(id string)) []search.Item {
	lq := strings.ToLower(q)
	items := []search.Item{{Title: "Review selected items", Subtitle: "Confirm the items to move to Trash", Icon: "trash-2", Tone: "red", Prominent: true,
		Actions: []search.Action{{Title: "Review", Run: func(h search.Host) { p.confirmReview(h, d, current) }}}}}
	for _, c := range d.Snapshot.Candidates {
		title := p.displayPath(c.Path)
		if lq != "" && search.Score(title, lq) == 0 {
			continue
		}
		id := c.ID
		items = append(items, search.Item{Title: title, Subtitle: c.Reason, Icon: iconForKind(c.Kind), Accessory: checkLabel(current[id]),
			Actions: []search.Action{{Title: "Toggle", Dismiss: search.DismissNever, Run: func(h search.Host) {
				toggle(id)
				h.Refresh()
			}}}})
	}
	for _, note := range d.Notes {
		items = append(items, search.Item{Title: note.Message, Icon: "info"})
	}
	return items
}

// iconForKind picks the row icon of a review item: the app bundle or a folder of data.
func iconForKind(kind string) string {
	if kind == apps.KindApp {
		return "app-window"
	}
	return "folder"
}

// confirmReview asks before anything moves. An empty selection moves nothing and says so.
func (p *App) confirmReview(h search.Host, d apps.Discovery, current map[string]bool) {
	chosen := apps.Selected(d.Snapshot, current)
	if len(chosen) == 0 {
		h.Toast(apps.NoSelectionMessage)
		return
	}
	title, message := apps.ConfirmText(chosen, p.displayPath)
	h.Push(palette.Confirm(title, message, func() { p.runUninstall(d.Snapshot, current) }))
}

// uninstallResultView shows what a run moved and what it left, with a way to see the Trash.
func (p *App) uninstallResultView(res apps.TrashResult) *palette.View {
	result := apps.BuildResult(res, p.displayPath)
	return &palette.View{
		Title: result.Title, Placeholder: "Uninstall result",
		Search: func(string) []search.Item {
			var items []search.Item
			if result.Success {
				items = append(items, search.Item{Title: result.Summary, Icon: "check", Tone: "green"})
			} else {
				if result.Message != "" {
					items = append(items, search.Item{Title: result.Message, Icon: "info"})
				}
				for _, d := range result.Details {
					items = append(items, search.Item{Title: d.Title, Subtitle: d.Subtitle, Icon: "file"})
				}
			}
			items = append(items, search.Item{Title: "Reveal in Trash", Subtitle: "Show the Trash folder", Icon: "trash-2",
				Actions: []search.Action{{Title: "Reveal in Trash", Dismiss: search.DismissNever, Run: func(search.Host) {
					_ = p.ports.Shell.Reveal(p.appsvc.TrashFolder())
				}}}})
			return items
		},
	}
}

// displayPath shortens the home folder to "~" for the review and the confirmation.
func (p *App) displayPath(path string) string {
	home, _ := os.UserHomeDir()
	return apps.DisplayPath(path, home)
}

// appsByPath indexes the apps by path, so a feature row can be mapped back to the app it describes.
func appsByPath(list []apps.App) map[string]apps.App {
	out := make(map[string]apps.App, len(list))
	for _, a := range list {
		out[a.Path] = a
	}
	return out
}
