package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// runDeclared performs a declarative action on the host with the host's own services. Each kind keeps the palette
// behavior of the native row it replaces: a set-query searches again and stays open, a path or an OS command hides the
// palette first, and a failed OS command reports its sentence as a toast.
func (s *Session) runDeclared(h search.Host, action api.Action) {
	switch action.Kind {
	case api.ActionSetQuery:
		h.SetQuery(action.Query)
	case api.ActionOpenPath:
		s.openPath(h, action.Path)
	case api.ActionRevealPath:
		s.revealPath(h, action.Path)
	case api.ActionSystem:
		s.runSystemOp(h, action.Op)
	case api.ActionQuickLook:
		s.fileOperation(h, s.deps.QuickLook, action.Path)
	case api.ActionOpenWith:
		s.openWithApp(h, action.Path, action.App)
	case api.ActionTrash:
		s.trashPaths(h, action.Paths)
	default:
		h.Toast(unavailableMessage)
	}
}

func (s *Session) openPath(h search.Host, path string) {
	if s.deps.OpenPath == nil {
		h.Toast(unavailableMessage)
		return
	}
	h.Hide()
	s.deps.OpenPath(path)
}

func (s *Session) revealPath(h search.Host, path string) {
	if s.deps.RevealPath == nil {
		h.Toast(unavailableMessage)
		return
	}
	h.Hide()
	s.deps.RevealPath(path)
}

// runUpdate performs an update operation of the host's updater. A refused operation shows its reason as a toast.
func (s *Session) runUpdate(h search.Host, op string) {
	if err := s.deps.RunUpdate(op); err != nil {
		h.Toast(err.Error())
	}
}

func (s *Session) runSystemOp(h search.Host, op string) {
	if s.deps.RunSystem == nil {
		h.Toast(unavailableMessage)
		return
	}
	h.Hide()
	s.deps.RunSystem(op, func(failure string) {
		if failure != "" {
			h.Toast(failure)
		}
	})
}

// fileOperation runs a file operation the host owns and reports its failure as a toast. The palette stays open: Quick
// Look is a preview, not a hand-off to another app.
func (s *Session) fileOperation(h search.Host, run func(string) error, path string) {
	if run == nil {
		h.Toast(unavailableMessage)
		return
	}
	if err := run(path); err != nil {
		h.Toast(err.Error())
	}
}

// openWithApp opens a file with the application at appPath. Opening hides the palette, as opening a path does.
func (s *Session) openWithApp(h search.Host, path, appPath string) {
	if s.deps.OpenWith == nil {
		h.Toast(unavailableMessage)
		return
	}
	h.Hide()
	if err := s.deps.OpenWith(path, appPath); err != nil {
		h.Toast(err.Error())
	}
}

// trashPaths moves paths to the Trash. The confirmation, when the action carries one, has already been answered.
func (s *Session) trashPaths(h search.Host, paths []string) {
	if s.deps.Trash == nil {
		h.Toast(unavailableMessage)
		return
	}
	for _, path := range paths {
		if err := s.deps.Trash(path); err != nil {
			h.Toast(err.Error())
			return
		}
	}
}
