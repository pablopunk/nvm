package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// accessoryActions are the keyboard way to the view's search accessory, which the pointer alone can open: Next and
// Previous step its options with Command+] and Command+[, and the action menu lists them. Every view that declares an
// accessory gets them from the host, so no extension has to remember to make its picker reachable from the keyboard.
func (s *Session) accessoryActions() []search.Action {
	spec := s.view.SearchAccessory
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
		{Title: "Next " + name, Mods: palette.Mod, Key: ui.KeyBracketRight, Label: palette.KeyLabel("⌃]"), Run: step(1), Section: viewGroupTitle, MenuOnly: true},
		{Title: "Previous " + name, Mods: palette.Mod, Key: ui.KeyBracketLeft, Label: palette.KeyLabel("⌃["), Run: step(-1), Section: viewGroupTitle, MenuOnly: true},
	}
}
