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
type extensionSearch struct{ p *App }

func extensionProvider(p *App) search.Provider { return extensionSearch{p: p} }

func (e extensionSearch) ID() string { return "extensions" }

// Root lists the commands of the running extensions. Nothing shows while extensions load.
func (e extensionSearch) Root(ctx context.Context) []search.Item {
	m := e.p.extensionManager()
	if m == nil {
		return nil
	}
	var out []search.Item
	for _, result := range m.Commands(ctx) {
		out = append(out, commandRowsFor(e.p, m, result)...)
	}
	return out
}

// Search emits the rows of every running extension for q, in one batch.
func (e extensionSearch) Search(ctx context.Context, q string, emit func(search.Batch)) {
	m := e.p.extensionManager()
	if m == nil {
		emit(search.Batch{Final: true})
		return
	}
	var out []search.Item
	for _, result := range m.Search(ctx, q) {
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
		out = append(out, extensionRow(p, m, result.ExtensionID, result.Title, item))
	}
	return out
}

// commandRowsFor lists a running extension's commands. A failed extension declares none, so the root shows nothing
// for it: its failure lives in the Extensions manager, and a failure row never ranks at the root.
func commandRowsFor(p *App, m *manager.Manager, result manager.CommandResult) []search.Item {
	if result.Failure != nil {
		return nil
	}
	var out []search.Item
	for _, command := range result.Commands {
		out = append(out, search.Item{
			Kind: result.Title, Title: command.Title, Subtitle: command.Subtitle, Icon: iconOr(command.Icon, "puzzle"),
			Tone: "purple", Score: scoreExtension,
			Actions: []search.Action{extview.ActionFor(p.extensionViewDeps(m), result.ExtensionID, api.Run(command.ID, "Run", nil))},
		})
	}
	return out
}

func extensionRow(p *App, m *manager.Manager, extID, title string, item api.Item) search.Item {
	row := search.Item{
		Kind: title, Title: item.Title, Subtitle: item.Subtitle, Icon: iconOr(item.Icon, "puzzle"), Tone: "purple",
		Score: scoreExtension,
	}
	if item.Primary != nil {
		row.Actions = append(row.Actions, extview.ActionFor(p.extensionViewDeps(m), extID, *item.Primary))
	}
	for _, action := range item.Actions {
		row.Actions = append(row.Actions, extview.ActionFor(p.extensionViewDeps(m), extID, action))
	}
	return row
}

func iconOr(icon, fallback string) string {
	if icon != "" {
		return icon
	}
	return fallback
}

func orDefault(text, fallback string) string {
	if text != "" {
		return text
	}
	return fallback
}
