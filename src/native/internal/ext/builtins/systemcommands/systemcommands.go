// Package systemcommands is the built-in that finds the operating system's commands (lock, sleep, log out, restart,
// shut down, empty the trash, open the settings) by title or keyword and runs them through the host. Destructive
// commands ask first with the copy the palette has always shown.
//
// Layer: ext. The package imports api, search and the standard library.
package systemcommands

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/commandmatch"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. The root lists its rows at the place the native system commands had, so the app asks for them by it.
const ID = "nevermind.system-commands"

const keywordMatchScore = commandmatch.KeywordScore

// Extension is the system commands built-in.
type Extension struct{}

// Manifest identifies the built-in. Its rows are listed under the Commands section, as the native rows were.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: ID, Title: "System Commands"}}
}

// SearchItems lists the supported commands that match the query. Commands show only for a query, never at the empty root.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	lq := strings.ToLower(query)
	if lq == "" {
		return nil, nil
	}
	system := ctx.System()
	var out []api.Item
	for _, c := range catalog(system.Labels().SettingsCommand) {
		if !system.HasCapability(c.capability) {
			continue
		}
		if score := matchScore(c, lq); score > 0 {
			out = append(out, commandItem(c, score))
		}
	}
	return out, nil
}

// matchScore ranks a command for the lowercased query: its title's score, or the keyword band when only a keyword
// contains the query. Zero means no match.
func matchScore(c command, lq string) int {
	return commandmatch.Score(c.title, c.keywords, lq)
}

// commandItem is the row of one command: its title, subtitle and tone, an Open action that runs it, and its usage key.
func commandItem(c command, score int) api.Item {
	item := api.Item{
		ID: c.op, Title: c.title, Subtitle: c.subtitle, Icon: c.icon, Score: float64(score),
		Section: search.KindCommands, LearnKey: "cmd:" + c.title,
		Primary: &api.Action{Kind: api.ActionSystem, Op: c.op, Title: "Open", Confirm: confirmationOf(c)},
	}
	if c.tone != "" {
		item.Appearance = &api.Appearance{Foreground: c.tone}
	}
	return item
}

// confirmationOf is the question a destructive command asks, as the palette's confirmation page shows it.
func confirmationOf(c command) *api.Confirmation {
	if !c.destructive {
		return nil
	}
	return &api.Confirmation{Title: c.title + "?", ConfirmLabel: c.title, Message: c.confirm, Destructive: true}
}
