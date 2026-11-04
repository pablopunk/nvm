package main

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.drag is the dev fixture of dragging files out of the palette. The list's rows carry a file, or a folder through
// an open-path action; the grid's cells carry image files, one with a separate drag picture. The paths are the fixture's
// own dev files under /tmp/nvm-drag: create them to try a drag, as a press that moves past the threshold starts one.

// dragTile is the picture a grid cell shows, a 1x1 PNG as a data URL.
const dragTile = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

var (
	dragListView = api.ListView("Drag list",
		api.Item{ID: "notes", Title: "notes.md", Subtitle: "A file row", File: &api.FileRef{Path: "/tmp/nvm-drag/notes.md", Name: "notes.md"}},
		api.Item{ID: "folder", Title: "Drag folder", Subtitle: "A folder row, through its open-path action",
			Primary: ptr(api.OpenPath("Open", "/tmp/nvm-drag"))},
		api.Item{ID: "relative", Title: "Relative path", Subtitle: "Relative paths never drag",
			File: &api.FileRef{Path: "notes.md", Name: "notes.md"}},
	)
	dragGridView = api.GridView("Drag grid", 3,
		api.Item{ID: "shot", Title: "shot.png", Image: &api.Image{Src: dragTile},
			File: &api.FileRef{Path: "/tmp/nvm-drag/shot.png", Name: "shot.png", Kind: "image"}},
		api.Item{ID: "clip", Title: "clip.mov", Image: &api.Image{Src: dragTile},
			File:      &api.FileRef{Path: "/tmp/nvm-drag/clip.mov", Name: "clip.mov", Kind: "video"},
			DragImage: &api.FileRef{Path: "/tmp/nvm-drag/clip-frame.png", Name: "clip-frame.png", Kind: "image"}},
	)
)

func ptr[T any](v T) *T { return &v }

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "list", Title: "Drag list fixture"},
		{ID: "grid", Title: "Drag grid fixture"},
	}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "list":
		return api.Result{View: &dragListView}, nil
	case "grid":
		return api.Result{View: &dragGridView}, nil
	}
	return api.Result{}, nil
}
