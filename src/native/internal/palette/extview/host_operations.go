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
