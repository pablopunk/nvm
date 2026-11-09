package app

import (
	"context"
	"sort"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform"
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

// OwnsCallBudgets lets the manager enforce each extension deadline independently of the aggregate snapshot budget.
func (e extensionSearch) OwnsCallBudgets() {}

func (e extensionSearch) ID() string { return "extensions" }

// Root collects what RootStream emits. The coordinator streams the root, so this is for callers that want it whole.
func (e extensionSearch) Root(ctx context.Context) []search.Item {
	var out []search.Item
	e.RootStream(ctx, func(batch search.Batch) {
		if batch.Replace {
			out = nil
		}
		out = append(out, batch.Items...)
	})
	return out
}

// RootStream emits the durable commands at once, then each extension's rows as that extension answers. Nothing shows
// while extensions load, and a slow extension never holds back the rows of the others.
func (e extensionSearch) RootStream(ctx context.Context, emit func(search.Batch)) {
	m := e.p.extensionManager()
	if m != nil {
		commands := e.commandRows(m, "")
		rows := map[string][]search.Item{}
		for _, result := range e.p.rootRows.cachedOutside(nil) {
			rows[result.ExtensionID] = rootRowsFor(e.p, m, result, e.in.shortcuts)
		}
		emit(search.Batch{Items: mergedExtensionRows(commands, rows), Replace: true})
		e.p.rootRows.stream(ctx, m, func(result manager.SearchResult) {
			rows[result.ExtensionID] = rootRowsFor(e.p, m, result, e.in.shortcuts)
			emit(search.Batch{Items: mergedExtensionRows(commands, rows), Replace: true})
		})
	}
	emit(search.Batch{Final: true})
}

func mergedExtensionRows(commands []search.Item, rows map[string][]search.Item) []search.Item {
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := append([]search.Item(nil), commands...)
	for _, id := range ids {
		out = append(out, rows[id]...)
	}
	return out
}

// Search emits, for q, the matching commands at once and then each extension's search rows as that extension answers. A
// command is found by its title, its aliases or its keywords, as a built-in command is by its title.
func (e extensionSearch) Search(ctx context.Context, q string, emit func(search.Batch)) {
	m := e.p.extensionManager()
	if m != nil {
		emit(search.Batch{Items: e.commandRows(m, q)})
		m.SearchEach(ctx, q, func(result manager.SearchResult) {
			if isCommandPlacedBuiltIn(result.ExtensionID) {
				return // listed by commandRows, at the place of its command
			}
			emit(search.Batch{Items: searchRowsFor(e.p, m, q, result, e.in.shortcuts)})
		})
	}
	emit(search.Batch{Final: true})
}

// commandRows lists the known commands that q finds, or the commands the root lists for the empty query. It never waits
// on an extension: a command not yet known appears once it is learned.
func (e extensionSearch) commandRows(m *manager.Manager, q string) []search.Item {
	var out []search.Item
	for _, result := range m.CachedCommands() {
		out = append(out, matchingCommandRows(e.p, m, result, q, e.in.shortcuts, e.in.aliases)...)
	}
	return out
}

// searchRowsFor turns one extension's answer into rows. A failed extension contributes at most its pointer row
// (see failureRows), never its items.
func searchRowsFor(p *App, m *manager.Manager, q string, result manager.SearchResult, bound map[string]string) []search.Item {
	if result.Failure != nil {
		return failureRows(p, result.ExtensionID, result.Title, q, result.Failure)
	}
	var out []search.Item
	for _, item := range result.Items {
		out = append(out, searchRow(p, m, result, item, bound))
	}
	return out
}

// scoreQueryFallback is the band of a query-fallback row: the 1.x fallback band (840 on its scale) lies between a
// substring match (50 here) and a scattered-letter match (20). A learned boost cannot lift the row past 50 (search.Boosted).
const scoreQueryFallback = 40

// searchRow is one search answer as a root row. A query fallback takes the fallback band whatever Score it declares.
func searchRow(p *App, m *manager.Manager, result manager.SearchResult, item api.Item, bound map[string]string) search.Item {
	row := extensionRow(p, m, result.ExtensionID, result.Title, item, bound)
	if item.QueryFallback {
		row.Score = scoreQueryFallback
	}
	return row
}

// rootRowsFor turns one extension's empty-query rows into rows. A failed extension contributes none at the root, as
// for its commands.
func rootRowsFor(p *App, m *manager.Manager, result manager.SearchResult, bound map[string]string) []search.Item {
	if result.Failure != nil {
		return nil
	}
	var out []search.Item
	for _, item := range result.Items {
		row := extensionRow(p, m, result.ExtensionID, result.Title, item, bound)
		if !m.IsBuiltIn(result.ExtensionID) {
			row.Score = scoreRootCommand
		}
		out = append(out, row)
	}
	return out
}

func extensionRow(p *App, m *manager.Manager, extID, title string, item api.Item, bound map[string]string) search.Item {
	item = extview.WithPreviewActions(item)
	item = extview.WithRevealShortcut(item, platform.ShellLabelsFor(p.goos).RevealInFileManager)
	deps := p.extensionViewDeps(m)
	row := search.Item{
		Kind: title, Title: item.Title, Subtitle: item.Subtitle, Icon: iconOr(item.Icon, "puzzle"), Tone: "purple",
		Score: scoreExtension, LastUsed: item.LastUsedMs, Detail: extview.DetailFor(deps, extID, item.Detail),
		QueryFallback: item.QueryFallback, Answers: item.AnswersQuery,
		DragPaths: api.DragPaths(item), DragImage: api.DragImagePath(item),
	}
	if m.IsBuiltIn(extID) {
		row.ID = item.ID
		row = builtInRow(row, item)
	} else {
		row.LearnKey = folderLearnKey(extID, item.LearnKey)
		if item.ID != "" {
			row.ID = folderLearnKey(extID, "item:"+item.ID)
			if row.LearnKey == "" {
				row.LearnKey = row.ID
			}
		}
	}
	row.Actions = extview.MenuActionsFor(deps, extID, extview.MenuSections(item.Primary, item.ActionPanel, item.Actions))
	if item.ActionPanelVisibility == api.PanelHidden {
		row.Actions = extview.HideFromMenu(row.Actions)
	}
	if m.IsBuiltIn(extID) && item.CustomizeAs != "" {
		p.shortcutTitles.remember(item.CustomizeAs, item.Title)
		row = p.withShortcutActions(row, item.CustomizeAs, bound)
	}
	return p.withAuthoringActions(row, extID, "")
}

// folderLearnKey namespaces a folder extension's learning key by its id, so its rows learn apart from built-in rows
// and from other extensions that choose the same key. An empty key stays empty: the row is not learned.
func folderLearnKey(extID, key string) string {
	if key == "" {
		return ""
	}
	return "ext:" + extID + ":" + key
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
	row.TintTitle = item.TintTitle
	row.TiePriority = item.TiePriority
	row.Prominent = item.Appearance != nil && item.Appearance.Background == "accent"
	if item.File != nil && item.Image == nil && item.Icon == "" {
		row.Icon = ""
		row.IconPath = item.File.Path
	}
	return row
}
