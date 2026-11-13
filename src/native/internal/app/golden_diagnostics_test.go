package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// fixedDiagnostics is the support records with one set of records and reporting state, so a golden shows the state its
// fixture names.
type fixedDiagnostics struct {
	records   []api.DiagnosticRecord
	reporting string
}

func (f fixedDiagnostics) Recent() []api.DiagnosticRecord { return f.records }

func (f fixedDiagnostics) Export(ref string) (string, bool) {
	return fmt.Sprintf("Reference: %s\n", ref), true
}

func (f fixedDiagnostics) ReportProblem() string { return "NVM-NEW01" }
func (f fixedDiagnostics) Reporting() string     { return f.reporting }

// diagnosticsGoldenRecords is three records, newest first: a failed background call, a clipboard failure and a problem report.
func diagnosticsGoldenRecords() []api.DiagnosticRecord {
	at := time.Date(2026, time.November, 3, 9, 41, 7, 0, time.UTC)
	return []api.DiagnosticRecord{
		{Ref: "NVM-K7P2Q", Component: "calc.rates", Time: at, Message: "rates source answered 503", Attributes: map[string]string{"attempt": "2"}},
		{Ref: "NVM-3HD9M", Component: "clipboard.poll", Time: at.Add(-95 * time.Minute), Message: "clipboard unavailable", Attributes: map[string]string{"panic": "true"}},
		{Ref: "NVM-WXR4T", Component: api.DiagnosticsProblemComponent, Time: at.Add(-3 * time.Hour)},
	}
}

// diagnosticsGoldenStates are the fixtures the goldens render: records in each reporting state, and an empty page.
var diagnosticsGoldenStates = []struct {
	name    string
	surface fixedDiagnostics
}{
	{name: "list", surface: fixedDiagnostics{records: diagnosticsGoldenRecords(), reporting: api.DiagnosticsRequested}},
	{name: "local-only", surface: fixedDiagnostics{records: diagnosticsGoldenRecords(), reporting: api.DiagnosticsStored}},
	{name: "empty", surface: fixedDiagnostics{reporting: api.DiagnosticsOff}},
}

// TestDiagnosticsPageGoldens renders the Diagnostics built-in's page in each reporting state the fixtures list, and its
// action menu on the first record.
func TestDiagnosticsPageGoldens(t *testing.T) {
	for _, state := range diagnosticsGoldenStates {
		p := goldenPalette(t)
		p.exts.mgr.Store(newFixtureManagerWithDiagnostics(t, p, state.surface))
		goldenOpen(p, p.diagnosticsPage())
		golden.AssertMatrix(t, "diagnostics-"+state.name, p.View, paletteGoldens)
	}
	p := goldenPalette(t)
	p.exts.mgr.Store(newFixtureManagerWithDiagnostics(t, p, diagnosticsGoldenStates[0].surface))
	goldenOpen(p, p.diagnosticsPage())
	p.Choose(0)
	p.OpenActionMenu(0)
	golden.AssertMatrix(t, "diagnostics-actions", p.View, paletteGoldens)
}
