// Example: a command that opens a list view of recent downloads. The command is durable (aliases,
// a global shortcut); the rows are produced per open and are never registered as commands.
package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

type downloads struct{}

var (
	_ api.Extension       = downloads{}
	_ api.CommandProvider = downloads{}
	_ api.ActionHandler   = downloads{}
)

func (downloads) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: "acme.downloads", Title: "Downloads"},
		Capabilities: []api.Capability{api.CapabilityDesktopFiles},
	}
}

func (downloads) Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID:             "recent-downloads",
		Title:          "Recent Downloads",
		Icon:           "download",
		Aliases:        []string{"dl"},
		GlobalShortcut: "CommandOrControl+Alt+D",
		Placement:      []api.Placement{api.PlacementRoot},
	}}, nil
}

func (downloads) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "recent-downloads":
		return recentDownloadsView(ctx)
	case "reveal-folder":
		return api.Result{}, ctx.Desktop().Files().Reveal("~/Downloads")
	}
	return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
}

func recentDownloadsView(ctx api.Context) (api.Result, error) {
	files, err := ctx.Desktop().Files().Find([]string{"~/Downloads"}, api.FindOptions{
		Limit:  40,
		Depth:  1,
		SortBy: "added",
		Order:  "desc",
	})
	if err != nil {
		return api.Result{}, err
	}
	items := make([]api.Item, 0, len(files))
	for _, file := range files {
		items = append(items, api.Item{
			ID:       file.Path,
			Title:    file.Name,
			Subtitle: file.DisplayPath,
			File:     &file,
			Primary:  api.Primary(api.OpenPath("Open", file.Path)),
			Actions:  []api.Action{api.RevealPath(ctx.System().Labels().RevealInFileManager, file.Path)},
		})
	}
	view := api.ListView("Recent Downloads", items...)
	view.Refresh = &api.Refresh{Immediate: true, Mode: api.PatchReplace, Ref: &api.ActionRef{ID: "recent-downloads"}}
	view.EmptyView = &api.EmptyState{Title: "No downloads yet", Subtitle: "Files added to Downloads appear here."}
	view.ActionPanel = api.Panel(api.Section("Folder",
		api.Run("reveal-folder", "Show Downloads Folder", nil)))
	return api.Result{View: &view, Navigation: api.NavPush}, nil
}
