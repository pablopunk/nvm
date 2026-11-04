package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionsView lists the installed extensions from the manager, with their state, and the actions that
// create, reload, enable, show and delete them. The review and manager views of WP3.2 build on it.
func (p *App) extensionsView() *palette.View {
	return &palette.View{
		Title: "Extensions", Placeholder: "Search Extensions",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			out := []search.Item{
				{Title: "Create Extension with AI", Subtitle: "Describe it; an extension is written, checked and installed", Icon: "zap", Tone: "yellow", Prominent: true,
					Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { h.Push(p.builderView()) }}}},
				{Title: "Reload Extensions", Icon: "rotate-ccw", Tone: "gray",
					Actions: []search.Action{{Title: "Reload", Dismiss: search.DismissNever, Run: func(h search.Host) {
						p.reloadExtensions(h)
					}}}},
			}
			m := p.extensionManager()
			if m == nil {
				return out
			}
			for _, l := range m.List() {
				if lq != "" && !strings.Contains(strings.ToLower(l.Title), lq) {
					continue
				}
				out = append(out, extensionListRow(p, m, l))
			}
			return out
		},
	}
}

func extensionListRow(p *App, m *manager.Manager, l manager.Listing) search.Item {
	state := extensionStateLabel(l)
	toggle := "Enable"
	if l.Enabled {
		toggle = "Disable"
	}
	return search.Item{
		ID: l.ID, Title: l.Title, Subtitle: l.ID + " · " + state, Icon: "puzzle", Tone: "purple", Accessory: state,
		Actions: []search.Action{
			{Title: toggle, Dismiss: search.DismissNever, Run: func(h search.Host) { setExtensionEnabled(p, m, l.ID, !l.Enabled, h) }},
			{Title: "Show Source in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) {
				h.Hide()
				mygo.Shell.ShowItemInFolder(l.Dir)
			}},
			{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
				if err := m.Uninstall(l.ID); err != nil {
					h.Toast(err.Error())
				}
				h.Refresh()
			}},
		},
	}
}

func setExtensionEnabled(p *App, m *manager.Manager, id string, enable bool, h search.Host) {
	var err error
	if enable {
		err = m.Enable(id)
	} else {
		err = m.Disable(id)
	}
	if err != nil {
		h.Toast(err.Error())
	}
	h.Refresh()
}

// extensionStateLabel says what an extension is doing in one word, and is the row's accessory.
func extensionStateLabel(l manager.Listing) string {
	switch {
	case len(l.Problems) > 0:
		return "Invalid"
	case !l.Enabled:
		return "Disabled"
	}
	switch l.Runtime.State {
	case supervisor.StateRunning:
		return "Running"
	case supervisor.StateBroken:
		return "Broken"
	case supervisor.StateDisabled:
		return "Disabled after failures"
	case supervisor.StateFailed:
		return "Restarting"
	}
	return "Starting"
}

// reloadExtensions rescans the folder and reports the count. It runs on the UI thread, so the scan is the only
// work: children start in the background.
func (p *App) reloadExtensions(h search.Host) {
	m := p.extensionManager()
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
