package app

import (
	diagnosticsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/diagnostics"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// diagnosticsPage is the Recent Errors page the root's Show Recent Errors command opens. The Diagnostics built-in declares
// its rows.
func (p *App) diagnosticsPage() *palette.View {
	fallback := &palette.View{Title: "Recent Errors", Placeholder: "Search recent errors", EmptyTitle: "No recent errors recorded", Search: func(string) []search.Item { return nil }}
	return p.builtInOpenPage(diagnosticsbuiltin.ID, fallback)
}

// reportProblem files a problem report through the Diagnostics built-in. The toast names the reference, and the page lists
// the recent errors where it appears.
//
//lint:ignore U1000 Retained for direct callers that need a friendly failure message.
func (p *App) reportProblem(h search.Host) {
	result, ok := p.builtInRun(diagnosticsbuiltin.ID, diagnosticsbuiltin.ReportAction)
	if !ok {
		h.Toast("The problem report could not be recorded")
		return
	}
	if result.Toast != nil {
		h.Toast(result.Toast.Message)
	}
	if result.View != nil {
		h.Push(p.builtInPalette(diagnosticsbuiltin.ID, *result.View))
	}
}
