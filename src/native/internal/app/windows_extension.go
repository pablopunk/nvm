package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// extensionWindowInvocation is the invocation an extension's window call acts on. While the palette is open, or an
// action the palette started is still running, that is the window the palette was summoned from, and the palette hands
// the keyboard back as its own commands do. Otherwise a shortcut fired, and the call acts on the app in front without
// taking focus.
func (p *App) extensionWindowInvocation(ctx context.Context) windowmgmt.Invocation {
	if p.paletteRuns.Load() > 0 || (p.Palette != nil && p.Palette.Visible()) {
		record := p.windowFocus.load()
		return windowmgmt.Invocation{Source: windowmgmt.SourcePalette, Target: targetIfFound(record.target, record.known)}
	}
	target, found := p.windows.FrontmostTarget(ctx)
	return windowmgmt.Invocation{Source: windowmgmt.SourceShortcut, Target: targetIfFound(target, found)}
}

// beginPaletteRun records the window the palette was summoned from, on the UI thread before the palette hides, and marks
// a palette run as started.
func (p *App) beginPaletteRun() {
	p.windowTarget()
	p.paletteRuns.Add(1)
}

func (p *App) endPaletteRun() {
	p.paletteRuns.Add(-1)
}

func targetIfFound(target windowmgmt.Target, found bool) *windowmgmt.Target {
	if !found {
		return nil
	}
	return &target
}
