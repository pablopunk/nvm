package app

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionsView is the Extensions list: the create and reload commands, then one row per installed extension.
func (p *App) extensionsView() *palette.View {
	return p.extensionsViewFor(p.currentExtensionController)
}

// extensionsViewFor builds the list from the controller ctl returns at each search, so the list follows a manager
// that starts after the view opened.
func (p *App) extensionsViewFor(ctl func() extensionController) *palette.View {
	return &palette.View{
		Title: "Extensions", Placeholder: "Search Extensions",
		Search: func(q string) []search.Item {
			out := []search.Item{
				{Title: "Create Extension with AI", Subtitle: "Describe it; an extension is written, checked and installed", Icon: "zap", Tone: "yellow", Prominent: true,
					Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { h.Push(p.builderView()) }}}},
				{Title: "Reload Extensions", Icon: "rotate-ccw", Tone: "gray",
					Actions: []search.Action{{Title: "Reload", Dismiss: search.DismissNever, Run: func(h search.Host) {
						reloadExtensions(h, ctl())
					}}}},
			}
			m := ctl()
			if m == nil {
				return out
			}
			for _, l := range m.List() {
				if matchesExtensionQuery(l, q) {
					out = append(out, extensionListRow(p, m, l))
				}
			}
			return out
		},
	}
}

// matchesExtensionQuery is true when q is empty or appears in the extension's title or id.
func matchesExtensionQuery(l manager.Listing, q string) bool {
	lq := strings.ToLower(q)
	return lq == "" || strings.Contains(strings.ToLower(extensionTitle(l)), lq) || strings.Contains(strings.ToLower(l.ID), lq)
}

// reloadExtensions rescans the folder and reports the count. It runs on the UI thread, so the scan is the only
// work: children start in the background.
func reloadExtensions(h search.Host, m extensionController) {
	if m == nil {
		h.Toast("Extensions are still loading")
		return
	}
	if err := m.Refresh(); err != nil {
		h.Toast(err.Error())
		return
	}
	h.Toast(fmt.Sprintf("%d extensions", len(m.List())))
	h.Refresh()
}
