package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// runAdmittedHostAction checks a captured authority at each direct host-effect boundary.
func (s *Session) runAdmittedHostAction(h search.Host, r route, valid func() bool) {
	a := r.action
	admit := func() bool {
		if !valid() {
			h.Toast(unavailableMessage)
			return false
		}
		return true
	}
	switch a.Kind {
	case api.ActionOpenURL:
		if s.deps.OpenURL == nil {
			h.Toast(unavailableMessage)
			return
		}
		h.Hide()
		if admit() {
			s.deps.OpenURL(a.URL)
		}
	case api.ActionCopyText:
		if s.deps.CopyText == nil {
			h.Toast(unavailableMessage)
			return
		}
		if admit() {
			s.deps.CopyText(h, a.Text)
		}
	case api.ActionCopyImage:
		if s.deps.CopyImage == nil || a.Path == "" {
			h.Toast(unavailableMessage)
			return
		}
		if admit() {
			s.deps.CopyImage(h, r.extID, a.Path, r.identity)
		}
	case api.ActionOpenPath:
		if s.deps.OpenPath == nil {
			h.Toast(unavailableMessage)
			return
		}
		h.Hide()
		if admit() {
			s.deps.OpenPath(a.Path)
		}
	case api.ActionRevealPath:
		if s.deps.RevealPath == nil {
			h.Toast(unavailableMessage)
			return
		}
		h.Hide()
		if admit() {
			s.deps.RevealPath(a.Path)
		}
	case api.ActionSetQuery:
		if admit() {
			h.SetQuery(a.Query)
		}
	case api.ActionQuickLook:
		if admit() {
			s.fileOperation(h, s.deps.QuickLook, a.Path)
		}
	case api.ActionOpenWith:
		if s.deps.OpenWith == nil {
			h.Toast(unavailableMessage)
			return
		}
		h.Hide()
		if admit() {
			if err := s.deps.OpenWith(a.Path, a.App); err != nil {
				h.Toast(err.Error())
			}
		}
	case api.ActionTrash:
		if s.deps.Trash == nil {
			h.Toast(unavailableMessage)
			return
		}
		for _, path := range a.Paths {
			if !admit() {
				return
			}
			if err := s.deps.Trash(path); err != nil {
				h.Toast(err.Error())
				return
			}
		}
	case api.ActionSystem:
		if a.Op == "open-system-settings" && a.Setting != "" && s.deps.OpenSettingsPane != nil {
			h.Hide()
			if admit() {
				s.deps.OpenSettingsPane(a.Setting)
			}
			return
		}
		if s.deps.RunSystem == nil {
			h.Toast(unavailableMessage)
			return
		}
		h.Hide()
		if admit() {
			s.deps.RunSystem(a.Op, func(failure string) {
				if failure != "" {
					h.Toast(failure)
				}
			})
		}
	}
}
