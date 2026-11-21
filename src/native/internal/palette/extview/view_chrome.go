package extview

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// footerName is the footer's left text of a list: the view's Status while it has one, its title otherwise.
func (s *Session) footerName(*search.Item) string {
	if s.view.Status != "" {
		return s.view.Status
	}
	return s.view.Title
}

// tabCyclesAccessory is the key handler of a list whose search accessory cycles on Tab, or nil when it does not. Tab
// chooses the next option, wrapping after the last, and the choice runs OnChange as a picked option does.
func (s *Session) tabCyclesAccessory(accessory *palette.Accessory) func(c *ui.Context, h search.Host) bool {
	if accessory == nil || s.view.Filter == nil || !s.view.Filter.CycleOnTab {
		return nil
	}
	return func(c *ui.Context, h search.Host) bool {
		if !c.Shortcut(0, ui.KeyTab) {
			return false
		}
		accessory.Choose((accessory.SelectedIndex() + 1) % len(accessory.Options))
		h.Refresh()
		return true
	}
}
