package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

func (s *Session) previewDeclaredFile(h search.Host, r route) {
	if s.deps.PreviewFile == nil {
		h.Toast(unavailableMessage)
		return
	}
	s.deps.PreviewFile(r.extID, r.action.Path, r.identity, func(err error) {
		if err != nil {
			h.Toast(err.Error())
		}
	})
}

func actionSupported(deps Deps, action api.Action) bool {
	return (deps.OperationsSupported == nil || deps.OperationsSupported(action.Requires)) && (deps.ActionSupported == nil || deps.ActionSupported(action))
}

func sectionsHavePrimary(sections []MenuSection) bool {
	for _, section := range sections {
		if section.Primary {
			return true
		}
	}
	return false
}
