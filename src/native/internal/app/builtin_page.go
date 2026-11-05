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
	m := p.extensionManager()
	if m == nil {
		return fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), builtInPageTimeout)
	defer cancel()
	result, failure, err := m.Run(ctx, id, api.Invocation{Ref: api.ActionRef{ID: openActionID}})
	if err != nil || failure != nil || result.View == nil {
		return fallback
	}
	return extview.NewSession(p.extensionViewDeps(m), id, *result.View).Palette()
}

// openActionID is the action a built-in's page opens with.
const openActionID = "open"
