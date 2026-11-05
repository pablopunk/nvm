package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionListRow is one installed extension. Its state is the accessory, its id and any reason the subtitle, and
// its folder, capabilities and diagnostics the detail pane of the chosen row.
func extensionListRow(p *App, ctl extensionController, l manager.Listing) search.Item {
	return search.Item{
		ID: l.ID, Title: extensionTitle(l), Subtitle: extensionSubtitle(l), Icon: "puzzle", Tone: extensionTone(l),
		Accessory: extensionStateLabel(l), Detail: extensionDetailPane(l), Actions: extensionRowActions(p, ctl, l),
	}
}

// extensionRowActions orders the actions so Enter is always safe: a disabled extension opens its review, a failed
// one resets, and a running one shows its source. Turning an extension off is never the primary action.
func extensionRowActions(p *App, ctl extensionController, l manager.Listing) []search.Action {
	var actions []search.Action
	if extensionCanEnable(l) {
		actions = append(actions, search.Action{Title: "Enable", Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(p.extensionReviewView(ctl, l))
		}})
	}
	if extensionDisabledByFailures(l) {
		actions = append(actions, search.Action{Title: "Reset", Dismiss: search.DismissNever, Run: func(h search.Host) {
			setExtensionEnabled(ctl, l.ID, true, h)
		}})
	}
	actions = append(actions, search.Action{Title: "Show Source", Run: func(h search.Host) {
		showExtensionSource(ctl, l, h)
	}})
	if p.ownedByBuilder(l.ID) {
		actions = append(actions, search.Action{Title: "Edit with AI", Run: func(h search.Host) {
			h.Push(p.startedChatView("builder", "", l.ID))
		}})
	}
	if l.Enabled {
		actions = append(actions, search.Action{Title: "Disable", Dismiss: search.DismissNever, Run: func(h search.Host) {
			setExtensionEnabled(ctl, l.ID, false, h)
		}})
	}
	if l.Enabled && p.declaresMenuBar(l.ID) {
		actions = append(actions, p.menuBarVisibilityAction(l.ID))
	}
	return append(actions,
		search.Action{Title: "Show in Folder", Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) {
			h.Hide()
			mygo.Shell.ShowItemInFolder(l.Dir)
		}},
		search.Action{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
			deleteExtension(ctl, l.ID, h)
		}},
	)
}

func setExtensionEnabled(ctl extensionController, id string, enable bool, h search.Host) {
	var err error
	if enable {
		err = ctl.Enable(id)
	} else {
		err = ctl.Disable(id)
	}
	if err != nil {
		h.Toast(err.Error())
	}
	h.Refresh()
}

func showExtensionSource(ctl extensionController, l manager.Listing, h search.Host) {
	source, err := ctl.Source(l.ID)
	if err != nil {
		h.Toast(err.Error())
		return
	}
	h.Push(extensionSourceView(extensionTitle(l), source))
}

func deleteExtension(ctl extensionController, id string, h search.Host) {
	if err := ctl.Uninstall(id); err != nil {
		h.Toast(err.Error())
	}
	h.Refresh()
}
