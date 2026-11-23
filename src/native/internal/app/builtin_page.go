package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
)

// builtInPageTimeout bounds the call that builds a built-in's page. The built-in answers in process.
const builtInPageTimeout = 5 * time.Second

// builtInOpenPage is the page a built-in answers its open action with, drawn by the palette. Without the extension
// manager or a page, the fallback stands in.
func (p *App) builtInOpenPage(id string, fallback *palette.View) *palette.View {
	result, ok := p.builtInRun(id, openActionID)
	if !ok || result.View == nil {
		return fallback
	}
	return p.openBuiltInSession(extview.NewSession(p.extensionViewDeps(p.extensionManager()), id, *result.View, result.SnapshotEvidence))
}

// builtInRun runs one of a built-in's actions in process and returns its result. It reports false without the extension
// manager, or when the built-in refuses or fails.
func (p *App) builtInRun(id, actionID string) (api.Result, bool) {
	m := p.extensionManager()
	if m == nil {
		return api.Result{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), builtInPageTimeout)
	defer cancel()
	result, failure, err := m.Run(ctx, id, api.Invocation{Ref: api.ActionRef{ID: actionID}})
	if err != nil || failure != nil {
		return api.Result{}, false
	}
	return result, true
}

// builtInPalette draws a view a built-in produced, as the palette draws the built-in's pages.
func (p *App) builtInPalette(id string, view api.View) *palette.View {
	return p.openBuiltInSession(extview.NewSession(p.extensionViewDeps(p.extensionManager()), id, view))
}

// openBuiltInSession opens a built-in's page on the palette, so its refreshes run and redraw it while it shows, as an
// extension page pushed by an action does. Without a palette the page is drawn but never refreshed.
func (p *App) openBuiltInSession(session *extview.Session) *palette.View {
	if p.Palette == nil {
		return session.Palette()
	}
	return session.Open(p.Palette)
}

// openActionID is the action a built-in's page opens with.
const openActionID = "open"
