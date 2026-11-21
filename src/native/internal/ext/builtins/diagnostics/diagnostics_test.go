package diagnostics

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var recordedAt = time.Date(2026, time.November, 3, 9, 41, 7, 0, time.UTC)

// fakeDiagnostics holds the records the host would keep, the reporting state and the reference a report gets.
type fakeDiagnostics struct {
	api.Diagnostics
	records   []api.DiagnosticRecord
	texts     map[string]string
	reporting string
	reference string
}

func (d *fakeDiagnostics) Recent() []api.DiagnosticRecord { return d.records }

func (d *fakeDiagnostics) Export(ref string) (string, error) {
	text, ok := d.texts[ref]
	if !ok {
		return "", errors.New("no such record")
	}
	return text, nil
}

func (d *fakeDiagnostics) ReportProblem() string { return d.reference }
func (d *fakeDiagnostics) Reporting() string     { return d.reporting }

// fakeContext is the context the built-in runs with, over one fake set of records.
type fakeContext struct {
	api.Context
	diagnostics *fakeDiagnostics
}

func (c fakeContext) Diagnostics() api.Diagnostics { return c.diagnostics }

func fixtureDiagnostics(reporting string) *fakeDiagnostics {
	return &fakeDiagnostics{
		records: []api.DiagnosticRecord{
			{Ref: "NVM-K7P2Q", Component: "calc.rates", Time: recordedAt, Message: "rates source answered 503"},
			{Ref: "NVM-3HD9M", Component: "clipboard.poll", Time: recordedAt.Add(-95 * time.Minute), Message: "clipboard unavailable"},
			{Ref: "NVM-WXR4T", Component: api.DiagnosticsProblemComponent, Time: recordedAt.Add(-3 * time.Hour)},
		},
		texts: map[string]string{
			"NVM-K7P2Q": "Reference: NVM-K7P2Q\nError: rates source answered 503\n",
			"NVM-3HD9M": "Reference: NVM-3HD9M\nError: clipboard unavailable\n",
			"NVM-WXR4T": "Reference: NVM-WXR4T\n",
		},
		reporting: reporting,
	}
}

func runOpen(t *testing.T, d *fakeDiagnostics) api.Result {
	t.Helper()
	result, err := New().RunAction(fakeContext{diagnostics: d}, api.Invocation{Ref: api.ActionRef{ID: OpenAction}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if result.View == nil {
		t.Fatal("open returned no page")
	}
	return result
}

func TestRecentErrorsPageListsEveryRecordNewestFirstWithItsReportingLabel(t *testing.T) {
	view := *runOpen(t, fixtureDiagnostics(api.DiagnosticsRequested)).View
	if view.Title != "Recent Errors" || view.Placeholder != "Search recent errors" || view.Hint != "Technical details only" {
		t.Fatalf("page = %q, %q, %q", view.Title, view.Placeholder, view.Hint)
	}
	if len(view.Items) != 3 {
		t.Fatalf("rows = %d, want 3", len(view.Items))
	}
	if view.Items[0].ID != "NVM-K7P2Q" || view.Items[2].Title != "Problem report" {
		t.Fatalf("order or titles wrong: %q, %q", view.Items[0].ID, view.Items[2].Title)
	}
	if got := view.Items[0].Accessories[0].Text; got != "Reporting requested · remote availability unknown" {
		t.Fatalf("accessory = %q", got)
	}
	if got := view.Items[0].Subtitle; got != "NVM-K7P2Q · 2026-11-03 09:41:07" {
		t.Fatalf("subtitle = %q", got)
	}
}

func TestEachRowCopiesTheReferenceFirstAndTheSafeDiagnosticsSecond(t *testing.T) {
	view := *runOpen(t, fixtureDiagnostics(api.DiagnosticsStored)).View
	row := view.Items[0]
	if row.Primary == nil || row.Primary.Kind != api.ActionCopyText || row.Primary.Text != "NVM-K7P2Q" {
		t.Fatalf("primary = %+v", row.Primary)
	}
	if len(row.Actions) != 2 || row.Actions[0].Title != "Copy Error Reference" || row.Actions[1].Title != "Copy Safe Diagnostics" {
		t.Fatalf("actions = %+v", row.Actions)
	}
	if got := row.Actions[1].Text; got != "Reference: NVM-K7P2Q\nError: rates source answered 503\n" {
		t.Fatalf("safe diagnostics = %q", got)
	}
}

func TestReportingStateLabelsEveryRecordTheSameWay(t *testing.T) {
	cases := map[string]string{
		api.DiagnosticsOff:       "Not reported",
		api.DiagnosticsRequested: "Reporting requested · remote availability unknown",
		api.DiagnosticsStored:    "Stored on this computer",
	}
	for state, want := range cases {
		view := *runOpen(t, fixtureDiagnostics(state)).View
		if got := view.Items[1].Accessories[0].Text; got != want {
			t.Fatalf("%s: accessory = %q, want %q", state, got, want)
		}
	}
}

func TestEmptyRecorderShowsTheEmptyState(t *testing.T) {
	view := *runOpen(t, &fakeDiagnostics{reporting: api.DiagnosticsOff}).View
	if len(view.Items) != 0 {
		t.Fatalf("rows = %d, want none", len(view.Items))
	}
	if view.EmptyView == nil || view.EmptyView.Title != "No recent errors recorded" {
		t.Fatalf("empty view = %+v", view.EmptyView)
	}
}

func TestReportFilesAReferenceAndShowsItWithAToast(t *testing.T) {
	d := fixtureDiagnostics(api.DiagnosticsStored)
	d.reference = "NVM-NEW01"
	result, err := New().RunAction(fakeContext{diagnostics: d}, api.Invocation{Ref: api.ActionRef{ID: ReportAction}})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if result.Toast == nil || result.Toast.Message != "Problem report NVM-NEW01 recorded" {
		t.Fatalf("toast = %+v", result.Toast)
	}
	if result.View == nil || len(result.View.Items) != 3 {
		t.Fatalf("report did not show the page: %+v", result.View)
	}
}

func TestReportWithoutAReferenceSaysSoAndShowsNoPage(t *testing.T) {
	result, err := New().RunAction(fakeContext{diagnostics: fixtureDiagnostics(api.DiagnosticsStored)}, api.Invocation{Ref: api.ActionRef{ID: ReportAction}})
	if !errors.Is(err, api.ErrNotAvailable) {
		t.Fatalf("report: %v, want ErrNotAvailable", err)
	}
	if result.View != nil || result.Toast != nil {
		t.Fatalf("result = %+v", result)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	_, err := New().RunAction(fakeContext{diagnostics: fixtureDiagnostics(api.DiagnosticsStored)}, api.Invocation{Ref: api.ActionRef{ID: "clear"}})
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestReportCommandIsSearchableAndTargetsReportAction(t *testing.T) {
	commands, err := New().Commands(fakeContext{diagnostics: fixtureDiagnostics(api.DiagnosticsStored)})
	if err != nil || len(commands) != 1 {
		t.Fatalf("commands = %+v, err = %v", commands, err)
	}
	if commands[0].ID != ReportAction || len(commands[0].Placement) != 1 || commands[0].Placement[0] != api.PlacementSearch {
		t.Fatalf("command = %+v", commands[0])
	}
}
