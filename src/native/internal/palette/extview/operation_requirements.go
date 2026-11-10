package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
	"slices"
)

func itemSupported(deps Deps, item api.Item) bool {
	return deps.OperationsSupported == nil || deps.OperationsSupported(item.Requires)
}

// ItemMenuSections keeps row requirements on captured actions without changing the source snapshot.
func ItemMenuSections(item api.Item) []MenuSection {
	sections := MenuSections(item.Primary, item.ActionPanel, item.Actions)
	for i := range sections {
		for j := range sections[i].Actions {
			action := &sections[i].Actions[j]
			action.Requires = append(slices.Clone(item.Requires), action.Requires...)
		}
	}
	return sections
}

func (s *Session) withOperationGuard(mapped search.Action, action api.Action) search.Action {
	run := mapped.Run
	if run != nil {
		mapped.Run = func(h search.Host) {
			if !actionSupported(s.deps, action) {
				h.Toast("This action requires an operation unavailable on this system.")
				return
			}
			run(h)
		}
	}
	return mapped
}

func (s *Session) runRoute(h search.Host, r route) {
	if !actionSupported(s.deps, r.action) {
		h.Toast("This action requires an operation unavailable on this system.")
		return
	}
	if r.action.Confirm != nil {
		h.Push(s.confirmPage(r))
		return
	}
	s.invoke(h, r)
}
