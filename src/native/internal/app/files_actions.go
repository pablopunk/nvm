package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// actionsFor returns the actions of one file row, as 1.x lists them: Open first (Enter), then Preview Contents (Preview
// for a picture or video), then Show in Finder. Copy and Trash are not file row actions in 1.x, so they are not here.
func (s *fileService) actionsFor(r files.Result) []search.Action {
	path := r.Path
	actions := []search.Action{
		{Title: "Open " + r.Name, Run: func(h search.Host) {
			hideOrReport(h, files.OpenPath(s.shell, path))
		}},
		{Title: previewTitle(r), Label: "⌃Y", Mods: mod, Key: ui.KeyY, Run: func(h search.Host) {
			h.Push(s.previewView(r))
		}},
		{Title: revealTitle(), Run: func(h search.Host) {
			hideOrReport(h, files.Reveal(s.shell, path))
		}},
	}
	return actions
}

// previewTitle is the menu title of the preview action: a picture or video is previewed, anything else has its contents
// previewed.
func previewTitle(r files.Result) string {
	if r.Kind == files.KindImage || r.Kind == files.KindVideo {
		return "Preview"
	}
	return "Preview Contents"
}

// revealTitle is the 1.x title of the action that shows a file in the system file manager.
func revealTitle() string {
	switch runtime.GOOS {
	case "darwin":
		return "Show in Finder"
	case "windows":
		return "Show in File Explorer"
	}
	return "Show in File Manager"
}

// previewView is the full-palette preview of one file, opened with Ctrl+Y: the same picture or text as the preview
// pane, with the whole list area to itself.
func (s *fileService) previewView(r files.Result) *palette.View {
	return &palette.View{
		Title:       r.Name,
		Placeholder: r.Name,
		HideInput:   true,
		Body: func(c *ui.Context, _ search.Host) {
			s.drawPreview(c, r)
		},
	}
}
