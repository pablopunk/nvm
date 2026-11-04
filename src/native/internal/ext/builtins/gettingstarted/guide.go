package gettingstarted

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// guideView is the guide: its rows, with the palette shortcut the user set read from the shortcut table.
func guideView(ctx api.Context) api.View {
	rows := guideRows(ctx.Shortcuts().Palette().Label)
	items := make([]api.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, guideItem(row))
	}
	view := api.ListView("Getting Started", items...)
	view.Placeholder = "Search tips"
	return view
}

// guideItem maps one guide row onto a list row. Information rows have no action.
func guideItem(row guideRow) api.Item {
	item := api.Item{ID: row.ID, Title: row.Title, Subtitle: row.Subtitle, Icon: row.Icon, Appearance: &api.Appearance{Foreground: api.ColorNone}}
	switch row.action {
	case actionSetShortcut:
		set := api.Action{Kind: api.ActionRecordPaletteShortcut, Title: "Set Shortcut"}
		item.Primary = &set
	case actionFinish:
		finish := api.Run(finishActionID, "Done for now", nil)
		item.Primary = &finish
	}
	return item
}

// guideAction is what a guide row does when the user presses Enter on it.
type guideAction string

const (
	actionNone        guideAction = ""
	actionSetShortcut guideAction = "set-shortcut"
	actionFinish      guideAction = "finish"
)

// guideRow is one item of the guide.
type guideRow struct {
	ID, Title, Subtitle, Icon string
	action                    guideAction
}

// guideRows lists the guide's items. shortcut is the palette shortcut's label, or empty.
func guideRows(shortcut string) []guideRow {
	return []guideRow{
		{ID: "getting-started-find", Title: "Find a command", Subtitle: "Return to the palette, then search for an app, file, or command.", Icon: "search"},
		{ID: "getting-started-run", Title: "Run it", Subtitle: `Choose a result and press Enter. Try "Calculator" or "Open Downloads".`, Icon: "keyboard"},
		shortcutRow(shortcut),
		{ID: "getting-started-permissions", Title: "Permissions when needed", Subtitle: "A prompt appears only when a command needs access.", Icon: "shield-check"},
		{ID: "getting-started-done", Title: "Done for now", Subtitle: "Dismiss this guide and keep using the palette.", Icon: "check", action: actionFinish},
	}
}

// shortcutRow is the guide item about the optional palette shortcut: what it does, and the action to set it.
func shortcutRow(shortcut string) guideRow {
	row := guideRow{ID: "getting-started-shortcut", Icon: "keyboard", action: actionSetShortcut}
	if shortcut == "" {
		row.Title = "Open Nevermind from anywhere"
		row.Subtitle = "Optional: set a shortcut for instant access. Launching Nevermind from its app icon also opens the palette."
		return row
	}
	row.Title = "Keyboard shortcut set"
	row.Subtitle = shortcut + " opens Nevermind from anywhere. You can change it any time."
	return row
}
