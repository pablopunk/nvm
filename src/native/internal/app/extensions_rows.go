package app

import (
	"context"
	"time"

	"github.com/egoist/mygo"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionRunTimeout bounds one action an extension runs from the palette. Actions run off the UI thread.
const extensionRunTimeout = 10 * time.Second

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
			Actions: []search.Action{runAction(p, m, result.ExtensionID, api.ActionRef{ID: command.ID}, "Run")},
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
		row.Actions = append(row.Actions, paletteAction(p, m, extID, *item.Primary))
	}
	for _, action := range item.Actions {
		row.Actions = append(row.Actions, paletteAction(p, m, extID, action))
	}
	return row
}

// paletteAction maps one wire action to a palette action. Only run executes extension code; the others are
// host operations on the data the action carries.
func paletteAction(p *App, m *manager.Manager, extID string, action api.Action) search.Action {
	title := action.Title
	switch {
	case action.Kind == api.ActionRun && action.Ref != nil:
		return runAction(p, m, extID, *action.Ref, orDefault(title, "Run"))
	case action.Kind == api.ActionOpenURL && action.URL != "":
		url := action.URL
		return search.Action{Title: orDefault(title, "Open"), Run: func(h search.Host) { h.Hide(); _ = mygo.Shell.OpenExternal(url) }}
	case action.Kind == api.ActionCopyText:
		text := action.Text
		return search.Action{Title: orDefault(title, "Copy"), Run: func(h search.Host) { mygo.Clipboard.WriteText(text); h.Hide() }}
	}
	return search.Action{Title: orDefault(title, string(action.Kind)), Run: func(h search.Host) { h.Toast("This action is not available yet") }}
}

// runAction runs an extension's action off the UI thread and shows any failure as a toast. Result navigation
// is not applied yet; the action's effect is what the extension does.
func runAction(p *App, m *manager.Manager, extID string, ref api.ActionRef, title string) search.Action {
	return search.Action{Title: title, Run: func(h search.Host) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), extensionRunTimeout)
			defer cancel()
			_, failure, err := m.Run(ctx, extID, api.Invocation{Ref: ref})
			message := ""
			switch {
			case err != nil:
				message = err.Error()
			case failure != nil:
				message = failure.Message
			}
			if message != "" && p.panel != nil {
				p.panel.Update(func() { h.Toast(message) })
			}
		}()
	}}
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
