package extview

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// viewGroupTitle titles the group of the view's own actions in a row's menu, as 1.x's View section does.
const viewGroupTitle = "View"

// MenuSection is one group of an action menu. The primary action's group has no title, a panel's sections keep theirs,
// and the fallback actions of a menu without a panel have none.
type MenuSection struct {
	Title   string
	Actions []api.Action
}

// MenuSections is what an action menu lists under a primary action: the primary first, then the panel's sections, or
// the fallback actions when the panel has no section. An action the primary repeats is listed once, and a group left
// empty is dropped. The sections copy their actions, so the caller's panel is never changed.
func MenuSections(primary *api.Action, panel *api.ActionPanel, fallback []api.Action) []MenuSection {
	var sections []MenuSection
	if primary != nil {
		sections = append(sections, MenuSection{Actions: []api.Action{*primary}})
	}
	if panel != nil && len(panel.Sections) > 0 {
		for _, section := range panel.Sections {
			sections = append(sections, MenuSection{Title: section.Title, Actions: slices.Clone(section.Actions)})
		}
	} else {
		sections = append(sections, MenuSection{Actions: slices.Clone(fallback)})
	}
	if primary != nil {
		for i := 1; i < len(sections); i++ {
			sections[i].Actions = slices.DeleteFunc(sections[i].Actions, func(action api.Action) bool {
				return sameWireAction(*primary, action)
			})
		}
	}
	return slices.DeleteFunc(sections, func(section MenuSection) bool { return len(section.Actions) == 0 })
}

// MenuWireActions is the flat list of MenuSections, in the order the menu lists them.
func MenuWireActions(primary *api.Action, panel *api.ActionPanel, fallback []api.Action) []api.Action {
	return flattenSections(MenuSections(primary, panel, fallback))
}

func flattenSections(sections []MenuSection) []api.Action {
	var out []api.Action
	for _, section := range sections {
		out = append(out, section.Actions...)
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

// MenuActionsFor maps the sections of a menu outside a view, such as a root row's or a command's, to palette actions.
func MenuActionsFor(deps Deps, extID string, sections []MenuSection) []search.Action {
	var out []search.Action
	for _, section := range sections {
		for _, action := range section.Actions {
			mapped := ActionFor(deps, extID, action)
			mapped.Section = section.Title
			out = append(out, mapped)
		}
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

// sectionActions maps the sections of the menu of itemID to palette actions. The actions are numbered from first, and
// each takes the title of its group.
func (s *Session) sectionActions(itemID string, first int, sections []MenuSection) []search.Action {
	var out []search.Action
	for _, section := range sections {
		for _, action := range section.Actions {
			mapped := s.actionAt(itemID, first+len(out), action)
			mapped.Section = section.Title
			out = append(out, mapped)
		}
	}
	return out
}

// pageActions is the view's own menu, listed while no row is chosen, such as on an empty list.
func (s *Session) pageActions() []search.Action {
	out := s.sectionActions("", 0, MenuSections(nil, s.view.ActionPanel, s.view.Actions))
	out = append(out, s.accessoryActions()...)
	if s.viewHidesMenu() {
		return HideFromMenu(out)
	}
	return out
}
