package app

import (
	"runtime"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// keyboardShortcutsView is the Keyboard Shortcuts list: the palette hotkey and every command, each with the shortcut it
// has. Enter changes a row's shortcut, and its action menu removes the shortcut the user chose.
func (p *App) keyboardShortcutsView() *palette.View {
	return &palette.View{
		Title:       "Keyboard Shortcuts",
		Placeholder: "Search Keyboard Shortcuts",
		EmptyTitle:  "No shortcuts found.",
		Hint:        "↵ Change",
		Search:      p.shortcutRows,
	}
}

// paletteRecorderView changes the palette hotkey, as the Settings row does.
func (p *App) paletteRecorderView() *palette.View {
	return p.shortcutRecorderView(shortcuts.Entry{ActionID: shortcuts.PaletteID, Title: shortcuts.PaletteTitle})
}

// shortcutRows lists the entries whose title, shortcut or message contain the query.
func (p *App) shortcutRows(query string) []search.Item {
	needle := strings.ToLower(strings.TrimSpace(query))
	var rows []search.Item
	for _, entry := range p.shortcutsManager().Entries() {
		row := p.shortcutRow(entry)
		if needle == "" || strings.Contains(strings.ToLower(row.Title+" "+row.Subtitle+" "+row.Accessory), needle) {
			rows = append(rows, row)
		}
	}
	return rows
}

// shortcutRow maps an entry onto a row. The shortcut is the accessory. An inactive binding says why under the title,
// and an action without a shortcut says so.
func (p *App) shortcutRow(entry shortcuts.Entry) search.Item {
	row := search.Item{
		ID:        "shortcut:" + entry.ActionID,
		Title:     entry.Title,
		Icon:      "keyboard",
		Accessory: p.shortcutLabelOf(entry.Accelerator),
	}
	change := "Change Shortcut"
	switch {
	case entry.Inactive != "":
		row.Subtitle = "Not active: " + entry.Inactive
	case entry.Accelerator == "":
		row.Subtitle = "No shortcut"
		change = "Set Shortcut"
	}
	row.Actions = []search.Action{{Title: change, Run: func(h search.Host) { h.Push(p.shortcutRecorderView(entry)) }}}
	if entry.Removable {
		row.Actions = append(row.Actions, search.Action{Title: "Remove Shortcut", Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.removeShortcut(h, entry.ActionID)
		}})
	}
	return row
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

// paletteAccessory is the Settings accessory of the palette shortcut. A blank palette shortcut means none.
func (p *App) paletteAccessory(accelerator string) string {
	if accelerator == "" {
		return "None"
	}
	return p.shortcutLabelOf(accelerator)
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
