package files

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Shortcuts of a file row, as 1.x's platform modifier spells them: Preview is the row's own key, and Reveal runs on Enter
// with the modifier.
const (
	revealShortcut = "CommandOrControl+Enter"
)

// recentFiles lists the newest files of the index, of one kind when kind is set.
func recentFiles(ctx api.Context, limit int, kind string) []api.FileRef {
	return fileRefs(ctx.Desktop().Files().Recent(indexOptions(limit, kind)))
}

// searchFiles lists the index entries whose names match query, best first, of one kind when kind is set.
func searchFiles(ctx api.Context, limit int, query, kind string) []api.FileRef {
	return fileRefs(ctx.Desktop().Files().SearchIndex(query, indexOptions(limit, kind)))
}

func indexOptions(limit int, kind string) api.IndexOptions {
	return api.IndexOptions{FindOptions: api.FindOptions{Limit: limit, Kind: kind}}
}

// fileRefs describes indexed files as the file refs the rows and actions take.
func fileRefs(indexed []api.IndexedFile) []api.FileRef {
	out := make([]api.FileRef, 0, len(indexed))
	for _, file := range indexed {
		out = append(out, api.FileRef{
			Path: file.Path, Name: file.Name, DisplayPath: file.DisplayPath, Kind: file.Kind,
			Extension: file.Extension, URL: file.URL, FileURL: file.FileURL, VideoURL: file.VideoURL,
			Size: file.Size, MtimeMs: file.MtimeMs, BirthtimeMs: file.BirthtimeMs,
		})
	}
	return out
}

// fileItem is the row of one file: its name, its folder, its system icon or picture, and the actions a user takes on it.
// Enter opens the file. The row drags out as the file itself.
func fileItem(ctx api.Context, file api.FileRef, score float64) api.Item {
	open := api.OpenPath("Open "+file.Name, file.Path)
	return api.Item{
		ID: file.Path, Title: file.Name, Subtitle: file.DisplayPath, Section: search.KindFiles,
		File: &file, Detail: &api.Detail{File: &file}, Score: score, Primary: &open, Actions: fileActions(ctx, file),
	}
}

// fileActions are the row's menu in 1.x order: the preview (⌘Y), Quick Look where the system has it, and the reveal in
// the file manager.
func fileActions(ctx api.Context, file api.FileRef) []api.Action {
	labels := ctx.System().Labels()
	preview := api.PreviewFile(previewTitle(file), file)
	preview.Shortcut = api.PreviewShortcut
	actions := []api.Action{preview}
	actions = append(actions, api.QuickLook(labels.PreviewFile, file.Path))
	reveal := api.RevealPath(labels.RevealInFileManager, file.Path)
	reveal.Shortcut = revealShortcut
	return append(actions, reveal)
}

// previewTitle names the in-palette preview as 1.x does: "Preview" for pictures and videos, "Preview Contents" for
// everything else.
func previewTitle(file api.FileRef) string {
	if file.Kind == "image" || file.Kind == "video" {
		return "Preview"
	}
	return "Preview Contents"
}

// withThumbnails gives the first pictures of a list their picture from the files service, as the native rows did.
// Rows past thumbnailRows keep the system icon.
func withThumbnails(ctx api.Context, items []api.Item) []api.Item {
	shown := 0
	for i := range items {
		if items[i].File == nil || items[i].File.Kind != "image" || shown >= thumbnailRows {
			continue
		}
		if url, found := ctx.Desktop().Files().Thumbnail(items[i].File.Path); found {
			items[i].Image = &api.Image{Src: url, Alt: items[i].Title, Fit: api.FitContain}
			shown++
		}
	}
	return items
}
