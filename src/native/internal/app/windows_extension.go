package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// extensionWindowInvocation is the invocation an extension's window call acts on. While the palette is open that is
// the window the palette was summoned from, and the palette hands the keyboard back as its own commands do. Otherwise
// a shortcut fired, and the call acts on the app in front without taking focus.
func (p *App) extensionWindowInvocation(ctx context.Context) windowmgmt.Invocation {
	if p.Palette != nil && p.Palette.Visible() {
		target, found := p.windowTarget()
		return windowmgmt.Invocation{Source: windowmgmt.SourcePalette, Target: targetIfFound(target, found)}
	}
	target, found := p.windows.FrontmostTarget(ctx)
	return windowmgmt.Invocation{Source: windowmgmt.SourceShortcut, Target: targetIfFound(target, found)}
}

func targetIfFound(target windowmgmt.Target, found bool) *windowmgmt.Target {
	if !found {
		return nil
	}
	return &target
}
