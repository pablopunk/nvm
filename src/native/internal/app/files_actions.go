package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// actionsFor returns the actions of one file row, in the order of 1.x: Open first (Enter), then the preview, Quick
// Look where the system has it, and the reveal in the file manager; Copy and Trash follow. Trash asks first, in the
// palette. Open With is not offered: the Shell port cannot list the applications that open a file yet, and an
// unavailable capability is left out rather than shown disabled.
func (s *fileService) actionsFor(r files.Result) []search.Action {
	path := r.Path
	actions := []search.Action{
		{Title: "Open " + r.Name, Run: func(h search.Host) {
			hideOrReport(h, files.OpenPath(s.shell, path))
		}},
		{Title: previewTitle(r), Label: "⌃Y", Mods: mod, Key: ui.KeyY, Run: func(h search.Host) {
			h.Push(s.previewView(r))
		}},
	}
	if s.quickLook {
		actions = append(actions, search.Action{Title: s.labels.PreviewFile, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := files.QuickLook(s.shell, path); err != nil {
				h.Toast(err.Error())
			}
		}})
	}
	return append(actions, search.Action{Title: s.labels.RevealInFileManager, Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) {
		hideOrReport(h, files.Reveal(s.shell, path))
	}})
}

// previewTitle names the in-palette preview as 1.x does: "Preview" for pictures and videos, "Preview Contents" for
// everything else.
func previewTitle(r files.Result) string {
	if r.Kind == files.KindImage || r.Kind == files.KindVideo {
		return "Preview"
	}
	return "Preview Contents"
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
