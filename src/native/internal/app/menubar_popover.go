package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/menubar/extitems"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
)

// popoverState is the popover page the palette shows: the item it belongs to, and its view. The palette leaves the page
// when it shows its root again, so the page counts as open only while it is on top.
type popoverState struct {
	key  string
	view *palette.View
}

// popoverHost opens the popovers of the menu bar items in the palette. 1.x opens each view in a window under its icon.
// MyGo's tray flyout needs the tray's own handle, which the platform port does not expose, so the palette shows the view
// instead: the closest match this port allows.
type popoverHost struct{ p *App }

// Toggle opens the popover of an item, or closes it when it is the page on top.
func (h popoverHost) Toggle(extensionID, barID string, popover api.Popover) {
	key := popoverKey(extensionID, barID)
	if h.showing(key) {
		h.p.popover = popoverState{}
		h.p.Hide()
		return
	}
	view := h.view(extensionID, popover)
	if view == nil {
		return
	}
	h.p.Root()
	h.p.popover = popoverState{key: key, view: view}
	h.p.Open(view)
	h.p.focusPanel()
}

// Refresh swaps in the popover's new view while it is open, so the page follows the extension's renders.
func (h popoverHost) Refresh(extensionID, barID string, popover api.Popover) {
	if !h.showing(popoverKey(extensionID, barID)) {
		return
	}
	view := h.view(extensionID, popover)
	if view == nil || !h.p.ReplaceIf(h.p.popover.view, view) {
		return
	}
	h.p.popover.view = view
}

func (h popoverHost) showing(key string) bool {
	return h.p.popover.key == key && h.p.popover.view != nil && h.p.Visible() && h.p.Top() == h.p.popover.view
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
