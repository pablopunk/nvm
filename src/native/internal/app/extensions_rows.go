package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionSearch is the root's extension source: rows from every running extension's SearchItems, and the
// durable commands the extensions declare. Each extension answers within its own budget (see package manager),
// so the coordinator never waits on a hanging extension for longer than that budget.
type extensionSearch struct {
	p  *App
	in *rootInput
}

func extensionProvider(p *App, in *rootInput) search.Provider { return extensionSearch{p: p, in: in} }

func (e extensionSearch) ID() string { return "extensions" }

// Root lists the rows and the commands of the running extensions. Nothing shows while extensions load.
func (e extensionSearch) Root(ctx context.Context) []search.Item {
	m := e.p.extensionManager()
	if m == nil {
		return nil
	}
	var out []search.Item
	for _, result := range e.p.rootRows.results(ctx, m) {
		out = append(out, rootRowsFor(e.p, m, result)...)
	}
	for _, result := range m.Commands(ctx) {
		out = append(out, commandRowsFor(e.p, m, result, e.in.shortcuts)...)
	}
	return out
}

// Search emits, for q, the commands of the running extensions that match it and their search rows, in one batch. A
// command is found by its title, its aliases or its keywords, as a built-in command is by its title.
func (e extensionSearch) Search(ctx context.Context, q string, emit func(search.Batch)) {
	m := e.p.extensionManager()
	if m == nil {
		emit(search.Batch{Final: true})
		return
	}
	var out []search.Item
	for _, result := range m.SearchCommands(ctx) {
		out = append(out, matchingCommandRows(e.p, m, result, q, e.in.shortcuts, e.in.aliases)...)
	}
	for _, result := range m.Search(ctx, q) {
		if isCommandPlacedBuiltIn(result.ExtensionID) {
			continue // listed by commandRows, at the place of its command
		}
		out = append(out, searchRowsFor(e.p, m, q, result)...)
	}
	emit(search.Batch{Items: out, Final: true})
}

// searchRowsFor turns one extension's answer into rows. A failed extension contributes at most its pointer row
// (see failureRows), never its items.
func searchRowsFor(p *App, m *manager.Manager, q string, result manager.SearchResult) []search.Item {
	if result.Failure != nil {
		return failureRows(p, result.ExtensionID, result.Title, q, result.Failure)
	}
	var out []search.Item
	for _, item := range result.Items {
		out = append(out, searchRow(p, m, result, item))
	}
	return out
}

// scoreQueryFallback is the band of a query-fallback row: the 1.x fallback band (840 on its scale) lies between a
// substring match (50 here) and a scattered-letter match (20). A learned boost cannot lift the row past 50 (search.Boosted).
const scoreQueryFallback = 40

// searchRow is one search answer as a root row. A query fallback takes the fallback band whatever Score it declares.
func searchRow(p *App, m *manager.Manager, result manager.SearchResult, item api.Item) search.Item {
	row := extensionRow(p, m, result.ExtensionID, result.Title, item)
	if item.QueryFallback {
		row.Score = scoreQueryFallback
	}
	return row
}

// rootRowsFor turns one extension's empty-query rows into rows. A failed extension contributes none at the root, as
// for its commands.
func rootRowsFor(p *App, m *manager.Manager, result manager.SearchResult) []search.Item {
	if result.Failure != nil {
		return nil
	}
	var out []search.Item
	for _, item := range result.Items {
		out = append(out, extensionRow(p, m, result.ExtensionID, result.Title, item))
	}
	return out
}

// commandRowsFor lists a running extension's commands. A failed extension declares none, so the root shows nothing
// for it: its failure lives in the Extensions manager, and a failure row never ranks at the root.
func commandRowsFor(p *App, m *manager.Manager, result manager.CommandResult, bound map[string]string) []search.Item {
	return matchingCommandRows(p, m, result, "", bound, nil)
}

// matchingCommandRows lists a running extension's commands whose title matches q as a phrase, or every command
// for the empty query. A failed extension declares none. Each command takes a keyboard shortcut from its action menu.
func matchingCommandRows(p *App, m *manager.Manager, result manager.CommandResult, q string, bound map[string]string, aliases map[string][]string) []search.Item {
	if result.Failure != nil {
		return nil
	}
	var out []search.Item
	for _, command := range result.Commands {
		actionID := extensionCommandShortcutID(result.ExtensionID, command.ID)
		if q != "" && !manager.CommandMatches(command, q) && aliasMatchScore(aliases[actionID], q) == 0 {
			continue
		}
		p.shortcutTitles.remember(actionID, command.Title)
		row := p.withShortcutActions(search.Item{
			Kind: result.Title, Title: command.Title, Subtitle: command.Subtitle, Icon: iconOr(command.Icon, "puzzle"),
			Tone: "purple", Score: scoreExtension,
			Actions: []search.Action{extview.ActionFor(p.extensionViewDeps(m), result.ExtensionID, api.Run(command.ID, command.Title, nil))},
		}, actionID, bound)
		out = append(out, p.withAuthoringActions(row, result.ExtensionID, command.ID))
	}
	return out
}

func extensionRow(p *App, m *manager.Manager, extID, title string, item api.Item) search.Item {
	deps := p.extensionViewDeps(m)
	row := search.Item{
		Kind: title, Title: item.Title, Subtitle: item.Subtitle, Icon: iconOr(item.Icon, "puzzle"), Tone: "purple",
		Score: scoreExtension, LastUsed: item.LastUsedMs, Detail: extview.DetailFor(deps, extID, item.Detail),
		QueryFallback: item.QueryFallback, Answers: item.AnswersQuery,
	}
	if m.IsBuiltIn(extID) {
		row.ID = item.ID
		row = builtInRow(row, item)
	}
	if item.Primary != nil {
		row.Actions = append(row.Actions, extview.ActionFor(deps, extID, *item.Primary))
	}
	for _, action := range item.Actions {
		row.Actions = append(row.Actions, extview.ActionFor(deps, extID, action))
	}
	return p.withAuthoringActions(row, extID, "")
}

func iconOr(icon, fallback string) string {
	if icon != "" {
		return icon
	}
	return fallback
}

// builtInRow applies the host-only presentation of a built-in's row: its section, tone, rank and learning key, which
// a folder extension cannot set. A built-in row with no tone has none, as its native row had.
func builtInRow(row search.Item, item api.Item) search.Item {
	if item.Section != "" {
		row.Kind = item.Section
	}
	row.Tone = ""
	if item.Appearance != nil {
		row.Tone = string(item.Appearance.Foreground)
	}
	if item.Score != 0 {
		row.Score = int(item.Score)
	}
	row.LearnKey = item.LearnKey
	return row
}
