package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	paldiag "github.com/pablopunk/nvm/native/internal/palette/diagnostics"
	"github.com/pablopunk/nvm/native/internal/search"
)

// goldenCopy is the copy the diagnostics goldens run: the clipboard is not touched.
func goldenCopy(search.Host, string) {}

// TestDiagnosticsPageGoldens renders the Recent Errors page in each reporting state the fixtures list, and its
// action menu on the first record.
func TestDiagnosticsPageGoldens(t *testing.T) {
	for _, state := range paldiag.States() {
		p := goldenPalette(t)
		reporting := state.Reporting
		goldenOpen(p, paldiag.View(state.Records, func() analytics.Reporting { return reporting }, goldenCopy))
		golden.AssertMatrix(t, "diagnostics-"+state.Name, p.View, paletteGoldens)
	}
	p := goldenPalette(t)
	goldenOpen(p, paldiag.View(paldiag.States()[0].Records, func() analytics.Reporting { return analytics.Requested }, goldenCopy))
	p.Choose(0)
	p.OpenActionMenu(0)
	golden.AssertMatrix(t, "diagnostics-actions", p.View, paletteGoldens)
}
