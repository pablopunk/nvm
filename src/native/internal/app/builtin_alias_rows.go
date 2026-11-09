package app

import (
	"runtime"
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/builtins/appcommands"
	placesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/places"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/systemcommands"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// builtInRowTitle is one row a command-placed built-in lists: its stable action ID and the title its search matches.
type builtInRowTitle struct {
	actionID string
	title    string
}

// builtInRowTitles lists the rows of a command-placed built-in in action ID order.
func builtInRowTitles(builtIn string) []builtInRowTitle {
	var out []builtInRowTitle
	switch builtIn {
	case systemcommands.ID:
		titles := systemcommands.Titles(platform.ShellLabelsFor(runtime.GOOS).SettingsCommand)
		for op, id := range systemRowActionIDs {
			out = append(out, builtInRowTitle{actionID: id, title: titles[op]})
		}
	case placesbuiltin.ID:
		for name, id := range placeRowActionIDs {
			out = append(out, builtInRowTitle{actionID: id, title: name})
		}
	case appcommands.ID:
		out = append(out, builtInRowTitle{actionID: shortcuts.CommandQuitNevermind, title: appcommands.QuitTitle})
		out = append(out, builtInRowTitle{actionID: shortcuts.CommandRestartNevermind, title: appcommands.RestartTitle})
	}
	slices.SortFunc(out, func(a, b builtInRowTitle) int { return strings.Compare(a.actionID, b.actionID) })
	return out
}

// builtInRootRows are the rows of one command-placed built-in for q, each with its shortcut and alias actions. A row an
// alias names is found even when its title does not match q: the built-in is searched again by that row's title, and
// only that row is kept.
func (p *App) builtInRootRows(builtIn, q string, in *rootInput) []search.Item {
	var out []search.Item
	found := map[string]bool{}
	for _, row := range builtInRows(p, builtIn, q) {
		if id := builtInRowActionID(builtIn, row); id != "" {
			row = p.withShortcutActions(row, id, in.shortcuts)
			found[id] = true
		}
		out = append(out, row)
	}
	if q == "" {
		return out
	}
	for _, entry := range builtInRowTitles(builtIn) {
		if found[entry.actionID] || aliasMatchScore(in.aliases[entry.actionID], q) == 0 {
			continue
		}
		for _, row := range builtInRows(p, builtIn, entry.title) {
			if builtInRowActionID(builtIn, row) == entry.actionID {
				out = append(out, p.withShortcutActions(row, entry.actionID, in.shortcuts))
				break
			}
		}
	}
	return out
}
