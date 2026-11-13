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
	return extview.NewSession(p.extensionViewDeps(p.extensionManager()), id, *result.View, result.SnapshotEvidence).Palette()
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
	return extview.NewSession(p.extensionViewDeps(p.extensionManager()), id, view).Palette()
}

// openActionID is the action a built-in's page opens with.
const openActionID = "open"
