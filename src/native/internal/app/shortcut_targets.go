package app

import (
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/apps"
	clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	dictbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
	emojibuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/emoji"
	notesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/notes"
	snippetsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	windowmanagementbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/windowmanagement"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// The prefixes of the action IDs that the shortcut document keys root rows by. Apps keep the 1.x "app:<path>" ID that
// shortcuts.AppActionID builds.
const (
	commandShortcutPrefix         = "command:"
	extensionShortcutPrefix       = "extension:"
	extensionActionShortcutPrefix = "extension-action:"
)

// shortcutTitles remembers the titles of the extension commands the root has listed, so the Keyboard Shortcuts list
// and conflict messages name a bound command without asking its extension. Search goroutines write it.
type shortcutTitles struct{ titles sync.Map }

func (s *shortcutTitles) remember(actionID, title string) { s.titles.Store(actionID, title) }

func (s *shortcutTitles) titleOr(actionID, fallback string) string {
	if title, ok := s.titles.Load(actionID); ok {
		return title.(string)
	}
	return fallback
}

// movedCommandShortcutActions are the commands that moved from the native list to a built-in. Their bindings keep the
// native keys, so the setup lists them under those keys as the native list did.
var movedCommandShortcutActions = []shortcuts.Action{
	{ID: shortcuts.CommandClipboardHistory, Title: "Clipboard History"},
}

// commandShortcutID names a palette command for the shortcut document by its stable ID. A command without one takes no
// shortcut.
func commandShortcutID(cmd command) string {
	return cmd.ID
}

// movedCommandShortcutIDs are the shortcut keys of the built-in commands that were native commands before. Their
// bindings keep the keys the user set, so these commands are keyed by the native IDs in place of their extension IDs.
var movedCommandShortcutIDs = map[string]string{
	extensionShortcutPrefix + apps.ID + ":" + apps.ForceQuitCommandID:                    shortcuts.CommandForceQuitApps,
	extensionShortcutPrefix + apps.ID + ":" + apps.QuitAllCommandID:                      shortcuts.CommandQuitAllApps,
	extensionShortcutPrefix + snippetsbuiltin.ID + ":" + snippetsbuiltin.ListCommandID:   shortcuts.CommandSnippets,
	extensionShortcutPrefix + notesbuiltin.ID + ":" + notesbuiltin.ListCommandID:         shortcuts.CommandQuickNotes,
	extensionShortcutPrefix + notesbuiltin.ID + ":" + notesbuiltin.NewWindowCommandID:    shortcuts.CommandNewFloatingNote,
	extensionShortcutPrefix + emojibuiltin.ID + ":" + emojibuiltin.CommandID:             shortcuts.CommandEmojiSymbols,
	extensionShortcutPrefix + dictbuiltin.ID + ":" + dictbuiltin.DictateCommandID:        shortcuts.CommandDictate,
	extensionShortcutPrefix + dictbuiltin.ID + ":" + dictbuiltin.HistoryCommandID:        shortcuts.CommandDictationHistory,
	extensionShortcutPrefix + clipboardbuiltin.ID + ":" + clipboardbuiltin.ListCommandID: shortcuts.CommandClipboardHistory,
}

// commandShortcutKey is the shortcut key of an extension command: the ActionID the command names, else its default key.
func commandShortcutKey(extID string, command api.Command) string {
	if command.ActionID != "" {
		return command.ActionID
	}
	return extensionCommandShortcutID(extID, command.ID)
}

func extensionCommandShortcutID(extID, commandID string) string {
	rest := extensionShortcutPrefix + extID + ":" + commandID
	if moved, ok := movedCommandShortcutIDs[rest]; ok {
		return moved
	}
	return rest
}

func splitExtensionCommandShortcutID(actionID string) (extID, commandID string, ok bool) {
	if extID, commandID, ok := splitMovedCommandShortcutID(actionID); ok {
		return extID, commandID, true
	}
	for _, prefix := range []string{extensionShortcutPrefix, extensionActionShortcutPrefix} {
		if rest, found := strings.CutPrefix(actionID, prefix); found {
			extID, commandID, ok := strings.Cut(rest, ":")
			return builtInIDOfShortcutName(extID), commandID, ok
		}
	}
	return "", "", false
}

// shortcutNamesOfBuiltIns maps the short extension names 1.x writes into shortcut keys to the built-ins' ids.
var shortcutNamesOfBuiltIns = map[string]string{
	"window-management": windowmanagementbuiltin.ID,
}

// builtInIDOfShortcutName is the id a shortcut key's extension name stands for: a built-in's id under its 1.x name, or the
// name itself.
func builtInIDOfShortcutName(name string) string {
	if id, ok := shortcutNamesOfBuiltIns[name]; ok {
		return id
	}
	return name
}

// splitMovedCommandShortcutID names the built-in command a moved shortcut key stands for.
func splitMovedCommandShortcutID(actionID string) (extID, commandID string, ok bool) {
	for key, moved := range movedCommandShortcutIDs {
		if moved == actionID {
			return strings.Cut(strings.TrimPrefix(key, extensionShortcutPrefix), ":")
		}
	}
	return "", "", false
}

// resolveShortcutTitle names a root action outside the Keyboard Shortcuts list's commands and reports whether it still
// exists. A source that is still loading (the app index, the extensions) says it does, so the shortcut binds at launch.
func (p *App) resolveShortcutTitle(actionID string) (string, bool) {
	if entry, ok := p.globalShortcuts.commands[actionID]; ok {
		return entry.command.Title, true
	}
	if path, ok := shortcuts.AppPathOf(actionID); ok {
		return p.appShortcutTitle(path)
	}
	if extID, commandID, ok := splitExtensionCommandShortcutID(actionID); ok {
		return p.shortcutTitles.titleOr(actionID, commandID), p.extensionMayRun(extID)
	}
	if cmd, ok := p.shortcutCommand(actionID); ok {
		return cmd.Title, true
	}
	if title, ok := strings.CutPrefix(actionID, commandShortcutPrefix); ok {
		_, found := p.builtInCommandRow(title)
		return title, found || p.extensionManager() == nil
	}
	return "", false
}

// appShortcutTitle names an app shortcut. It stays listed when the app is not installed now, as 1.x lists it.
func (p *App) appShortcutTitle(path string) (string, bool) {
	for _, a := range p.appsSnapshot() {
		if a.Path == path {
			return a.Name, true
		}
	}
	title, _ := shortcuts.DescribeApp(shortcuts.AppActionID(path))
	return title, true
}

// extensionMayRun reports whether an extension is built in or enabled, or whether the extensions are still loading.
func (p *App) extensionMayRun(extID string) bool {
	m := p.extensionManager()
	if m == nil || m.IsBuiltIn(extID) {
		return true
	}
	for _, listing := range m.List() {
		if listing.ID == extID {
			return listing.Enabled
		}
	}
	return false
}

// shortcutCommand finds the palette command, shown at the root or only for a query, that actionID names.
func (p *App) shortcutCommand(actionID string) (command, bool) {
	for _, cmd := range p.commands() {
		if commandShortcutID(cmd) == actionID {
			return cmd, true
		}
	}
	return command{}, false
}

// builtInCommandRow finds the row titled title among the built-ins the command list places, such as Lock Screen.
func (p *App) builtInCommandRow(title string) (search.Item, bool) {
	for _, id := range commandPlacedBuiltIns {
		for _, row := range builtInRows(p, id, title) {
			if row.Title == title && len(row.Actions) > 0 {
				return row, true
			}
		}
	}
	return search.Item{}, false
}

// runShortcutCommand runs the row a shortcut names, as choosing it from the root does. Apps open without the palette,
// and so do extension commands that run in place; every other row opens the palette and runs its first action.
func (p *App) runShortcutCommand(actionID string) {
	if path, ok := shortcuts.AppPathOf(actionID); ok {
		p.openShortcutApp(path)
		return
	}
	if entry, ok := p.globalShortcuts.commands[actionID]; ok && runsWithoutPalette(entry.command) {
		p.runExtensionCommandInPlace(entry)
		return
	}
	p.runShortcutInPalette(actionID)
}

// runShortcutInPalette opens the palette and runs the row actionID names, as choosing it from the root does.
func (p *App) runShortcutInPalette(actionID string) {
	if action, ok := p.shortcutAction(actionID); ok {
		title, _ := p.resolveShortcutTitle(actionID)
		p.Palette.OpenAction(title, action)
	}
}

func (p *App) openShortcutApp(path string) {
	for _, a := range p.appsSnapshot() {
		if a.Path == path {
			_ = p.ports.Shell.LaunchApp(shellApp(a))
			return
		}
	}
}

// shortcutAction is the first action of the row actionID names.
func (p *App) shortcutAction(actionID string) (search.Action, bool) {
	if entry, ok := p.globalShortcuts.commands[actionID]; ok {
		return extview.ActionFor(p.extensionViewDeps(p.extensionManager()), entry.extID, commandAction(entry.command)), true
	}
	if extID, commandID, ok := splitExtensionCommandShortcutID(actionID); ok {
		m := p.extensionManager()
		if m == nil {
			return search.Action{}, false
		}
		return extview.ActionFor(p.extensionViewDeps(m), extID, api.Run(commandID, "Run", nil)), true
	}
	if cmd, ok := p.shortcutCommand(actionID); ok {
		return search.Action{Title: cmd.Title, Run: p.trackedRun(cmd), Dismiss: cmd.Dismiss}, true
	}
	if title, ok := strings.CutPrefix(actionID, commandShortcutPrefix); ok {
		if row, found := p.builtInCommandRow(title); found {
			return row.Actions[0], true
		}
	}
	return search.Action{}, false
}
