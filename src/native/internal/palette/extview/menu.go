package extview

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// MenuWireActions is what an action menu lists under a primary action: the primary first, then the panel's actions, or
// the fallback actions when the panel has no section. An action the primary repeats is listed once.
func MenuWireActions(primary *api.Action, panel *api.ActionPanel, fallback []api.Action) []api.Action {
	secondary := fallback
	if panel != nil && len(panel.Sections) > 0 {
		secondary = panelActions(panel)
	}
	out := make([]api.Action, 0, len(secondary)+1)
	if primary != nil {
		out = append(out, *primary)
	}
	for _, action := range secondary {
		if primary == nil || !sameWireAction(*primary, action) {
			out = append(out, action)
		}
	}
	return out
}

// sameWireAction is true when two wire actions do the same thing, so a repeated action is listed once.
func sameWireAction(a, b api.Action) bool {
	if a.Kind != b.Kind || a.Title != b.Title {
		return false
	}
	if a.Ref != nil && b.Ref != nil {
		return a.Ref.ID == b.Ref.ID
	}
	return a.Ref == nil && b.Ref == nil && a.URL == b.URL && a.Path == b.Path && a.Text == b.Text
}

func panelActions(panel *api.ActionPanel) []api.Action {
	var out []api.Action
	for _, section := range panel.Sections {
		out = append(out, section.Actions...)
	}
	return out
}

// MenuActionsFor maps the actions of a menu outside a view, such as a root row's or a command's, to palette actions.
func MenuActionsFor(deps Deps, extID string, wire []api.Action) []search.Action {
	out := make([]search.Action, 0, len(wire))
	for _, action := range wire {
		out = append(out, ActionFor(deps, extID, action))
	}
	return out
}

// HideFromMenu marks actions that a hidden action panel keeps out of the Command-K menu. They keep their shortcuts.
func HideFromMenu(actions []search.Action) []search.Action {
	for i := range actions {
		actions[i].MenuHidden = true
	}
	return actions
}

// viewHidesMenu is true when the view's action panel is hidden: its actions run from their shortcuts only.
func (s *Session) viewHidesMenu() bool {
	return s.view.ActionPanelVisibility == api.PanelHidden
}

// viewActionsNotIn are the view's own actions that an item's menu does not already list.
func (s *Session) viewActionsNotIn(own []api.Action) []api.Action {
	var out []api.Action
	for _, action := range MenuWireActions(nil, s.view.ActionPanel, s.view.Actions) {
		if !slices.ContainsFunc(own, func(o api.Action) bool { return sameWireAction(o, action) }) {
			out = append(out, action)
		}
	}
	return out
}

// pageActions is the view's own menu, listed while no row is chosen, such as on an empty list.
func (s *Session) pageActions() []search.Action {
	wire := MenuWireActions(nil, s.view.ActionPanel, s.view.Actions)
	out := make([]search.Action, 0, len(wire))
	for i, action := range wire {
		out = append(out, s.actionAt("", i, action))
	}
	if s.viewHidesMenu() {
		return HideFromMenu(out)
	}
	return out
}
