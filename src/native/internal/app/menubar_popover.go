package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/menubar/extitems"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// menuPopover is the popover of one menu bar item: a palette that draws into a window under the item.
type menuPopover struct {
	palette *palette.Palette
}

// popoverHost opens the popovers of the menu bar items under their icons, as 1.x does: each is a window of the declared
// size that closes when the user presses outside it.
type popoverHost struct{ p *App }

// Toggle shows the popover of an item under its tray item, or closes it when it is shown.
func (h popoverHost) Toggle(extensionID, barID string, tray platform.Tray, popover api.Popover) {
	key := popoverKey(extensionID, barID)
	open := h.p.popovers[key]
	if open != nil && open.palette.Visible() {
		open.palette.Hide()
		return
	}
	view := h.view(extensionID, popover)
	if view == nil {
		return
	}
	if open == nil {
		open = h.create(key, tray, popover, view)
		if open == nil {
			return
		}
	}
	open.palette.SetStack(view)
	open.palette.Show()
}

// Refresh swaps in the popover's new view while it is shown, so it follows the extension's renders.
func (h popoverHost) Refresh(extensionID, barID string, _ platform.Tray, popover api.Popover) {
	open := h.p.popovers[popoverKey(extensionID, barID)]
	if open == nil || !open.palette.Visible() {
		return
	}
	view := h.view(extensionID, popover)
	if view == nil {
		return
	}
	open.palette.SetStack(view)
	open.palette.Refresh()
}

// create makes the popover window under the tray item and the palette that draws into it. It is kept until the item
// goes, so reopening keeps the palette's state.
func (h popoverHost) create(key string, tray platform.Tray, popover api.Popover, root *palette.View) *menuPopover {
	pal := palette.New(palette.Options{
		Root:    root,
		Clock:   searchClock,
		Icon:    h.p.fileIcon,
		Learner: h.p.learn,
		DragRow: h.p.dragRow,
	})
	window, err := tray.OpenPopover(platform.PopoverOptions{Width: popover.Width, Height: popover.Height}, ui.View(pal.View))
	if err != nil {
		logx.Default().Warn("menu bar popover unavailable", logx.F{"popover": key, "error": err.Error()})
		return nil
	}
	pal.Attach(window)
	if h.p.popovers == nil {
		h.p.popovers = map[string]*menuPopover{}
	}
	open := &menuPopover{palette: pal}
	h.p.popovers[key] = open
	return open
}

// view builds the palette page of a popover. It is nil while the extension host is not running.
func (h popoverHost) view(extensionID string, popover api.Popover) *palette.View {
	m := h.p.extensionManager()
	if m == nil {
		return nil
	}
	return extview.NewSession(h.p.extensionViewDeps(m), extensionID, popover.View).Palette()
}

func popoverKey(extensionID, barID string) string {
	return extensionID + "/" + barID
}

var _ extitems.PopoverHost = popoverHost{}
