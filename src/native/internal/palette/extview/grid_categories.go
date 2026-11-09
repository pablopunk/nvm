package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// categoryKeys steps the search accessory of a grid that declares CategoryKeys: Tab takes the next option and Shift+Tab
// the previous one, wrapping at the ends. The accessory's value changes and its OnChange runs, as a pick in its menu
// does, so the extension answers with the new category.
func (s *Session) categoryKeys(c *ui.Context, h search.Host) bool {
	switch {
	case c.Shortcut(0, ui.KeyTab):
		s.stepCategory(1)
	case c.Shortcut(ui.Shift, ui.KeyTab):
		s.stepCategory(-1)
	default:
		return false
	}
	h.Refresh()
	return true
}

// stepCategory moves the accessory by d options from its current value, and runs its change handler with the value
// it lands on. A value that names no option starts from the first.
func (s *Session) stepCategory(d int) {
	if s.accessory == nil || s.view.Filter == nil || s.view.Filter.OnChange == nil {
		return
	}
	options := s.view.Filter.Options
	if len(options) == 0 {
		return
	}
	next := options[nextCategoryIndex(options, s.accessory.Value, d)]
	s.accessory.Value = next.Value
	s.invokeInPlace(*s.view.Filter.OnChange, next.Value)
}

// nextCategoryIndex is the index d steps from the option whose value is current, wrapping around the list.
func nextCategoryIndex(options []api.ViewFilterOption, current string, d int) int {
	index := 0
	for i, option := range options {
		if option.Value == current {
			index = i
			break
		}
	}
	return (index + d + len(options)) % len(options)
}
