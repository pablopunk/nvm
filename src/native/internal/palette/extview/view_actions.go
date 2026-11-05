package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// viewActionKey names the view's own actions for run handles, as a row names its own.
const viewActionKey = "view"

// viewActions is the action menu of a page and its shortcuts: the view's actions, then the actions of its panel that the
// view does not list again. The rows keep their own actions.
func (s *Session) viewActions() []search.Action {
	wire := s.viewWireActions()
	out := make([]search.Action, 0, len(wire))
	for i, action := range wire {
		out = append(out, s.actionAt(viewActionKey, i, action))
	}
	return out
}

// viewWireActions lists the view's actions, then the panel's actions that are not among them.
func (s *Session) viewWireActions() []api.Action {
	wire := append([]api.Action(nil), s.view.Actions...)
	if s.view.ActionPanel == nil {
		return wire
	}
	for _, section := range s.view.ActionPanel.Sections {
		for _, action := range section.Actions {
			if !containsAction(wire, action) {
				wire = append(wire, action)
			}
		}
	}
	return wire
}

// containsAction is true when list already has an action that does the same thing as action.
func containsAction(list []api.Action, action api.Action) bool {
	for _, have := range list {
		if sameAction(have, action) {
			return true
		}
	}
	return false
}
