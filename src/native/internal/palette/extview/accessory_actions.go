package extview

import "github.com/pablopunk/nvm/native/internal/search"

// accessoryActions are the keyboard way to the view's filter, which the pointer alone can open: Next and Previous step
// its options, and the action menu lists them. Every view that declares a filter gets them from the host, so no extension
// has to remember to make its picker reachable from the keyboard. They bind no key: a punctuation key such as a bracket
// is not on the same place of every keyboard layout, and a shortcut that cannot be typed on one layout must not exist.
func (s *Session) accessoryActions() []search.Action {
	spec := s.view.Filter
	if s.accessory == nil || spec == nil || spec.OnChange == nil || len(spec.Options) < 2 {
		return nil
	}
	name := orDefault(spec.Tooltip, "Filter")
	step := func(d int) func(search.Host) {
		return func(h search.Host) {
			s.stepCategory(d)
			if h != nil {
				h.Refresh()
			}
		}
	}
	return []search.Action{
		{Title: "Next " + name, Run: step(1), Section: viewGroupTitle, MenuOnly: true},
		{Title: "Previous " + name, Run: step(-1), Section: viewGroupTitle, MenuOnly: true},
	}
}

// withAccessoryActions adds the accessory's actions to a menu that already has some. A row without actions stays
// without, so a list of plain rows does not grow an Actions footer only for its filter; the keys still step the picker
// through the view's key handler when it has one.
func (s *Session) withAccessoryActions(out []search.Action) []search.Action {
	if len(out) == 0 {
		return out
	}
	return append(out, s.accessoryActions()...)
}
