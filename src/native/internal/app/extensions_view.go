package app

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionsViewTitle names the Extensions list, so a refresh can tell it apart from the views pushed on it.
const extensionsViewTitle = "Extensions"

// extensionsView is the Extensions list: the create and reload commands, then one row per installed extension.
func (p *App) extensionsView() *palette.View {
	return p.extensionsViewFor(p.currentExtensionController)
}

// extensionsViewFor builds the list from the controller ctl returns at each search, so the list follows a manager
// that starts after the view opened. The rows are ranked by their titles for the query, so an exact title is first.
func (p *App) extensionsViewFor(ctl func() extensionController) *palette.View {
	return &palette.View{
		Title: extensionsViewTitle, Placeholder: "Search Extensions",
		Search: func(q string) []search.Item {
			rows := []search.Item{
				{Title: "Create Extension with AI", Subtitle: "Describe it; an extension is written, checked and installed", Icon: "zap", Tone: "yellow", Prominent: true,
					Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { h.Push(p.startedChatView("builder", "", "")) }}}},
				{Title: "Reload Extensions", Icon: "rotate-ccw", Tone: "gray",
					Actions: []search.Action{{Title: "Reload", Dismiss: search.DismissNever, Run: func(h search.Host) {
						reloadExtensions(h, ctl())
					}}}},
			}
			m := ctl()
			if m != nil {
				for _, l := range m.List() {
					rows = append(rows, extensionListRow(p, m, l))
				}
			}
			return search.RankScored(rows, q, search.ItemTexts)
		},
	}
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
