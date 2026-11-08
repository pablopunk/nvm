package extview

import "github.com/pablopunk/nvm/native/internal/search"

// toggleDeclaredSetting leaves the current view responsive while the extension's checked settings service saves.
func (s *Session) toggleDeclaredSetting(h search.Host, r route) {
	if s.deps.ToggleSetting == nil || r.action.Setting == "" {
		h.Toast(unavailableMessage)
		return
	}
	current := func() bool { return !s.dismissed }
	if loading, ok := h.(search.ActionLoadingHost); ok {
		current = loading.BeginAction()
	}
	s.deps.ToggleSetting(r.extID, r.action.Setting, r.identity, func(err error) {
		if !current() || s.dismissed {
			return
		}
		if err != nil {
			h.Toast(err.Error())
			return
		}
		h.Refresh()
	})
}
