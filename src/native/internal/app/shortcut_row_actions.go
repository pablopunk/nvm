package app

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/palette/aliasview"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// The action menu titles 1.x gives every customizable root row.
const (
	setShortcutTitle    = "Set keyboard shortcut"
	removeShortcutTitle = "Remove keyboard shortcut"
	setAliasTitle       = "Set alias"
)

// withShortcutActions gives a root row the customization actions of its action menu, after its own actions: Set
// keyboard shortcut opens the recorder, Remove keyboard shortcut is there while it has one, and Set alias opens the
// alias view. It also shows the shortcut at the end of the row. bound is the captured Accelerators of the manager,
// since rows are built off the UI thread.
func (p *App) withShortcutActions(it search.Item, actionID string, bound map[string]string) search.Item {
	if len(it.Actions) == 0 || it.Disabled {
		return it
	}
	accelerator := bound[actionID]
	if accelerator != "" && it.Accessory == "" {
		it.Accessory = shortcutLabelOf(accelerator)
	}
	entry := shortcuts.Entry{ActionID: actionID, Title: it.Title, Accelerator: accelerator}
	actions := append(slices.Clip(it.Actions), search.Action{Title: setShortcutTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(p.shortcutRecorderView(entry))
	}})
	if accelerator != "" {
		actions = append(actions, search.Action{Title: removeShortcutTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.removeShortcut(h, actionID)
		}})
	}
	actions = append(actions, search.Action{Title: setAliasTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
		h.Push(aliasview.View(aliasview.Target{ID: actionID, Title: it.Title}, p.aliasStore()))
	}})
	it.Actions = actions
	return it
}

// boundShortcuts captures the manager's shortcuts for the root's rows. It runs on the UI thread.
func (p *App) boundShortcuts() map[string]string {
	return p.shortcutsManager().Accelerators()
}
