package app

import (
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/palette/windows"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// The prefixes of the action IDs that the shortcut document keys root rows by. Window commands keep their own
// "window." IDs, and apps keep the 1.x "app:<path>" ID that shortcuts.AppActionID builds.
const (
	commandShortcutPrefix   = "command:"
	extensionShortcutPrefix = "extension:"
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

func shortcutCommandID(title string) string {
	return commandShortcutPrefix + title
}

// commandShortcutID names a palette command for the shortcut document: its own ID when it has one, else its title.
// Built-in rows that stand among the commands use their title too, as the native commands they replaced did.
func commandShortcutID(cmd command) string {
	if cmd.ID != "" {
		return cmd.ID
	}
	return shortcutCommandID(cmd.Title)
}

func extensionCommandShortcutID(extID, commandID string) string {
	return extensionShortcutPrefix + extID + ":" + commandID
}

func splitExtensionCommandShortcutID(actionID string) (extID, commandID string, ok bool) {
	rest, found := strings.CutPrefix(actionID, extensionShortcutPrefix)
	if !found {
		return "", "", false
	}
	return strings.Cut(rest, ":")
}

// resolveShortcutTitle names a root action outside the Keyboard Shortcuts list's commands and reports whether it still
// exists. A source that is still loading (the app index, the extensions) says it does, so the shortcut binds at launch.
func (p *App) resolveShortcutTitle(actionID string) (string, bool) {
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
	for _, cmd := range p.commands(false) {
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

// runShortcutCommand runs the root row a shortcut names, as choosing it from the root does. Window commands act on
// the app in front and apps open without the palette; every other row opens the palette and runs its first action.
func (p *App) runShortcutCommand(actionID string) {
	if windows.RunShortcut(p.windowDeps(), actionID) {
		return
	}
	if path, ok := shortcuts.AppPathOf(actionID); ok {
		p.openShortcutApp(path)
		return
	}
	if action, ok := p.shortcutAction(actionID); ok {
		p.Show()
		p.RunAction(action)
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
