package app

import (
	"runtime"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	shortcutsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/shortcuts"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// keyboardShortcutsPage is the Keyboard Shortcuts list the root command opens. The Keyboard Shortcuts built-in declares
// its rows, and the palette runs its change and remove actions through runShortcutAction.
func (p *App) keyboardShortcutsPage() *palette.View {
	fallback := &palette.View{Title: "Keyboard Shortcuts", Placeholder: "Search Keyboard Shortcuts", EmptyTitle: "No shortcuts found.", Hint: "↵ Change", Search: func(string) []search.Item { return nil }}
	return p.builtInOpenPage(shortcutsbuiltin.ID, fallback)
}

// paletteRecorderView changes the palette hotkey, as the Settings row does.
func (p *App) paletteRecorderView() *palette.View {
	return p.shortcutRecorderView(shortcuts.Entry{ActionID: shortcuts.PaletteID, Title: shortcuts.PaletteTitle})
}

// runShortcutAction performs a shortcut operation an action of the Keyboard Shortcuts built-in names. Record pushes the
// recorder of that action's shortcut, and remove drops the shortcut the user chose.
func (p *App) runShortcutAction(h search.Host, op, target string) {
	switch op {
	case api.ShortcutOpRecord:
		h.Push(p.shortcutRecorderView(p.shortcutEntry(target)))
	case api.ShortcutOpRemove:
		p.removeShortcut(h, target)
	}
}

// shortcutEntry is the table's entry for an action. An action the table does not list is named by its ID.
func (p *App) shortcutEntry(actionID string) shortcuts.Entry {
	for _, entry := range p.shortcutsManager().Entries() {
		if entry.ActionID == actionID {
			return entry
		}
	}
	return shortcuts.Entry{ActionID: actionID, Title: actionID}
}

func (p *App) removeShortcut(h search.Host, actionID string) {
	if err := p.shortcutsManager().RemoveAction(actionID); err != nil {
		h.Toast(err.Error())
		return
	}
	h.Toast("Shortcut removed")
	h.Refresh()
}

// labeler spells shortcuts for this platform, with the Hyper key the app holds.
func (p *App) labeler() shortcuts.Labeler {
	return shortcuts.Labeler{GOOS: runtime.GOOS, Hyper: p.hyperKey()}
}

// shortcutLabelOf spells a canonical accelerator for this platform, with ✦ for the Hyper key, or "" when there is none.
func (p *App) shortcutLabelOf(accelerator string) string {
	return p.labeler().Label(accelerator)
}

// paletteSettingAccessory is the Settings accessory of the palette shortcut, as 1.x shows it: nothing, or Not active
// when the OS refused the shortcut.
func (p *App) paletteSettingAccessory() string {
	if p.shortcutsManager().PaletteStatus().Unavailable() {
		return "Not active"
	}
	return ""
}

// paletteChangeMessage is the toast after the palette shortcut changes. A system feature that also holds the key is
// said out loud.
func (p *App) paletteChangeMessage(change shortcuts.PaletteChange) string {
	label := p.shortcutLabelOf(change.Accelerator)
	switch {
	case change.Unchanged:
		return "Shortcut unchanged: " + label
	case change.SystemOwner != "":
		return "Shortcut set: " + label + ". " + change.SystemOwner + " also uses it"
	}
	return "Shortcut set: " + label
}
