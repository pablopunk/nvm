package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// actionsFor returns the actions of one file row. The first runs on Enter; the rest are in the action menu. Trash asks
// first, in the palette. Open With is not offered: the Shell port cannot list the applications that open a file yet,
// and an unavailable capability is left out rather than shown disabled.
func (s *fileService) actionsFor(r files.Result) []search.Action {
	path := r.Path
	return []search.Action{
		{Title: "Open", Run: func(h search.Host) {
			h.Hide()
			s.report(h, files.OpenPath(s.shell, path))
		}},
		{Title: "Show in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) {
			h.Hide()
			s.report(h, files.Reveal(s.shell, path))
		}},
		{Title: "Preview", Label: "⌃Y", Mods: mod, Key: ui.KeyY, Run: func(h search.Host) {
			h.Push(s.previewView(r))
		}},
		{Title: "Copy Path", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(h search.Host) {
			h.Hide()
			s.report(h, s.clipboard.WriteText(path))
		}},
		{Title: "Copy File", Label: "⌃⇧C", Mods: mod | ui.Shift, Key: ui.KeyC, Run: func(h search.Host) {
			h.Hide()
			s.report(h, s.copyFiles(path))
		}},
		{Title: "Move to Trash", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Run: func(h search.Host) {
			h.Push(palette.Confirm("Move to Trash", r.Name, func() {
				if err := files.Trash(s.shell, s.home, path); err != nil {
					s.log.Warn("files.trash-failed", logx.F{"reason": err.Error()})
				}
			}))
		}},
	}
}

// report shows an action's error in the palette. An action that hid the palette first has its message shown the next
// time the palette opens, which is the best the palette can do without a native dialog.
func (s *fileService) report(h search.Host, err error) {
	if err != nil {
		h.Toast(err.Error())
	}
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
