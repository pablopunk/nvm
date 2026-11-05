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
	case api.ActionWindow:
		s.runWindow(h, action)
	default:
		h.Toast(unavailableMessage)
	}
}

// runWindow hands a window action to the host, after the palette hides so that the window takes the focus.
func (s *Session) runWindow(h search.Host, action api.Action) {
	if s.deps.Window == nil {
		h.Toast(unavailableMessage)
		return
	}
	h.Hide()
	s.deps.Window(s.extID, action)
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
