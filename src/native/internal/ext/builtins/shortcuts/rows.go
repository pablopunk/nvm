package shortcuts

import "github.com/pablopunk/nvm/native/internal/ext/api"

// sourceUser is the source of a shortcut the user chose. Only those can be removed.
const sourceUser = "user"

// shortcutRow is one bound shortcut: its label, and the actions that change it or remove the user's choice. Enter changes it.
func shortcutRow(record api.ShortcutRecord) api.Item {
	change := recordAction(record.ActionID)
	actions := []api.Action{change}
	if record.Source == sourceUser {
		actions = append(actions, removeAction(record.ActionID))
	}
	actions = append(actions, copyCommandAction(record.ActionID))
	return api.Item{
		ID: "shortcut:" + record.ActionID, Title: record.Title, Subtitle: record.Subtitle, Icon: "keyboard",
		Accessories: []api.Accessory{{Text: record.Label}},
		Primary:     api.Primary(change), Actions: actions,
	}
}

// recordAction opens the host's recorder for the action's shortcut.
func recordAction(actionID string) api.Action {
	return api.Action{Kind: api.ActionShortcut, Op: api.ShortcutOpRecord, Target: actionID, Title: "Change shortcut"}
}

// removeAction drops the shortcut the user chose for the action.
func removeAction(actionID string) api.Action {
	return api.Action{Kind: api.ActionShortcut, Op: api.ShortcutOpRemove, Target: actionID, Title: "Remove shortcut"}
}

// copyCommandAction copies the command line that runs the action, for a terminal or a compositor binding.
func copyCommandAction(actionID string) api.Action {
	return api.Action{Kind: api.ActionShortcut, Op: api.ShortcutOpCopyCommand, Target: actionID, Title: "Copy Command Line"}
}

// Title, subtitle and accessory of the palette's own shortcut, the first row of the list.
const (
	paletteRowTitle    = "Open Nevermind"
	paletteRowSubtitle = "Shows and hides the palette from anywhere"
	paletteUnsetLabel  = "Not set"
)

// paletteRow is the shortcut that opens Nevermind. It is listed first, set or not, and can be changed but not removed
// here: without it the palette has no way to open.
func paletteRow(record api.ShortcutRecord) api.Item {
	label := record.Label
	if label == "" {
		label = paletteUnsetLabel
	}
	change := recordAction(record.ActionID)
	return api.Item{
		ID: "shortcut:" + record.ActionID, Title: paletteRowTitle, Subtitle: paletteRowSubtitle, Icon: "keyboard",
		Accessories: []api.Accessory{{Text: label}},
		Primary:     api.Primary(change), Actions: []api.Action{change, copyCommandAction(record.ActionID)},
	}
}
