package app

import (
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

// matchingCommandRows lists a running extension's commands that q finds, or the commands the root lists for the empty
// query. A failed extension declares none. Each command takes a keyboard shortcut from its action menu, unless it is a
// built-in command that is not customizable.
func matchingCommandRows(p *App, m *manager.Manager, result manager.CommandResult, q string, bound map[string]string, aliases map[string][]string) []search.Item {
	if result.Failure != nil {
		return nil
	}
	var out []search.Item
	for _, command := range result.Commands {
		actionID := commandShortcutKey(result.ExtensionID, command)
		score, listed := commandRowScore(m, result.ExtensionID, command, q, aliases[actionID])
		if !listed {
			continue
		}
		p.shortcutTitles.remember(actionID, command.Title)
		row := commandRow(p, m, result, command, score)
		if commandTakesShortcuts(m, result.ExtensionID, command) {
			row = p.withShortcutActions(row, actionID, bound)
		}
		out = append(out, p.withAuthoringActions(row, result.ExtensionID, command.ID))
	}
	return out
}

// commandRow is the row of one command of an extension, at score. Its Run action runs the command.
func commandRow(p *App, m *manager.Manager, result manager.CommandResult, command api.Command, score int) search.Item {
	primary := commandAction(command)
	sections := extview.MenuSections(&primary, command.ActionPanel, command.Actions)
	deps := p.extensionViewDeps(m)
	row := search.Item{
		Kind: result.Title, Title: command.Title, Subtitle: commandSubtitle(command, result.Title), Icon: iconOr(command.Icon, "puzzle"),
		Tone: commandTone(command), Score: score,
		Actions: commandMenuActions(deps, result.ExtensionID, sections),
	}
	row.ID = folderLearnKey(result.ExtensionID, "command:"+command.ID)
	row.LearnKey = row.ID
	if m.IsBuiltIn(result.ExtensionID) {
		row = builtInCommandRow(row, command)
	}
	return row
}

func commandMenuActions(deps extview.Deps, extID string, sections []extview.MenuSection) []search.Action {
	actions := extview.MenuActionsFor(deps, extID, sections)
	if len(actions) > 0 && len(sections) > 0 && len(sections[0].Actions) > 0 {
		actions[0] = extview.CommandActionFor(deps, extID, sections[0].Actions[0])
		actions[0].Section = sections[0].Title
	}
	return actions
}

// commandRowScore is the score of a command's row for q, and whether the command's row is listed at all. The empty root
// lists the commands placed there. A built-in command is ranked as its 1.x action was. Other extensions' commands are
// found by their title, aliases or keywords and rank in the extension band.
func commandRowScore(m *manager.Manager, extID string, command api.Command, q string, aliases []string) (int, bool) {
	if q == "" {
		return rootCommandRowScore(m, extID, command), commandPlacedAtRoot(command)
	}
	if !commandPlacedInSearch(command) {
		return 0, false
	}
	if m.IsBuiltIn(extID) {
		return builtInCommandSearchScore(command, q, aliases)
	}
	if manager.CommandMatches(command, q) || aliasMatchScore(aliases, q) > 0 {
		return scoreExtension, true
	}
	return 0, false
}

// rootCommandRowScore is a command's score at the empty root. A built-in command scores as its 1.x action did: the score
// it names, or 0 when it names none. Unused folder commands share the ordinary command default and learn from use.
func rootCommandRowScore(m *manager.Manager, extID string, command api.Command) int {
	if m.IsBuiltIn(extID) {
		return int(command.Score)
	}
	return scoreRootCommand
}

// builtInCommandSearchScore ranks a built-in command for a typed query as 1.x ranks an action: by its title, then by its
// keywords and aliases. It never takes the extension band, which would put an AI command above the native rows it shares a
// prefix with (Triage above Clipboard History for "cl").
func builtInCommandSearchScore(command api.Command, q string, aliases []string) (int, bool) {
	lq := strings.ToLower(q)
	score := commandTitleScore(command.Title, strings.Join(command.Keywords, " "), false, lq)
	if score == 0 {
		score = aliasMatchScore(append(slices.Clone(command.Aliases), aliases...), lq)
	}
	return score, score > 0
}

// commandPlacedAtRoot reports whether a command's row shows at the empty root. Only commands with no placement or one
// that names the root do.
func commandPlacedAtRoot(command api.Command) bool {
	return len(command.Placement) == 0 || slices.Contains(command.Placement, api.PlacementRoot)
}

// commandPlacedInSearch reports whether a command's row answers a typed query. Hidden commands never do.
func commandPlacedInSearch(command api.Command) bool {
	if len(command.Placement) == 0 {
		return true
	}
	return slices.Contains(command.Placement, api.PlacementRoot) || slices.Contains(command.Placement, api.PlacementSearch)
}

// commandTakesShortcuts reports whether a command's row offers the shortcut and alias actions. Folder extensions keep
// them; a built-in's command offers them only when it is Customizable.
func commandTakesShortcuts(m *manager.Manager, extID string, command api.Command) bool {
	return !m.IsBuiltIn(extID) || command.Customizable
}

// commandTone is the tone of a command's row: the foreground the command names, else the extension tone. The neutral
// foreground takes no tone at all, as the native rows did.
func commandTone(command api.Command) string {
	if command.Appearance == nil || command.Appearance.Foreground == "" {
		return "purple"
	}
	if command.Appearance.Foreground == api.ColorNone {
		return ""
	}
	return string(command.Appearance.Foreground)
}

// builtInCommandRow applies the host-only presentation of a built-in's command: its section and learning key, and its
// own subtitle only, as the native command rows have. A built-in command with no section keeps its extension's title.
func builtInCommandRow(row search.Item, command api.Command) search.Item {
	if command.Section != "" {
		row.Kind = command.Section
	}
	row.Subtitle = command.Subtitle
	row.LearnKey = command.LearnKey
	return row
}

// commandAction is the action that runs a command. A command that asks to be dismissed after its run closes the palette
// as the run is dispatched, so the handler runs over the app the palette was summoned from.
func commandAction(command api.Command) api.Action {
	action := api.Run(command.ID, command.Title, nil)
	action.DismissAfterRun = command.DismissAfterRun
	action.Background = command.Mode == api.ModeBackground
	return action
}

// commandSubtitle is the subtitle a command row shows: the command's own, or the extension's title when the command has
// none, as 1.x shows it. The row's right-edge label already names the extension.
func commandSubtitle(command api.Command, extensionTitle string) string {
	if command.Subtitle != "" {
		return command.Subtitle
	}
	return extensionTitle
}
