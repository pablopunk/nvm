package diagnostics

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
)

// Static is a fixed set of records for goldens and tests. Its references are the ones it was given, so a render never
// depends on the random references a live recorder issues.
type Static struct {
	Items []diag.Record
}

// Recent returns the records, newest first, as the recorder does.
func (s Static) Recent() []diag.Record {
	return append([]diag.Record(nil), s.Items...)
}

// Export renders the record with the reference, as the recorder's export does.
func (s Static) Export(ref string) string {
	for _, record := range s.Items {
		if record.Ref == ref {
			return diag.FormatExport(record)
		}
	}
	return ""
}

// States are the fixture sets the goldens render: records in each reporting state, and an empty page.
func States() []State {
	return []State{
		{Name: "list", Records: Static{Items: fixtureRecords()}, Reporting: analytics.Requested},
		{Name: "local-only", Records: Static{Items: fixtureRecords()}, Reporting: analytics.LocalOnly},
		{Name: "empty", Records: Static{}, Reporting: analytics.Off},
	}
}

// State is one golden fixture: the records and the reporting state the page shows them with.
type State struct {
	Name      string
	Records   Records
	Reporting analytics.Reporting
}

// fixtureRecords is three records, newest first: a failed background call, a clipboard failure and a problem report.
func fixtureRecords() []diag.Record {
	at := time.Date(2026, time.November, 3, 9, 41, 7, 0, time.UTC)
	return []diag.Record{
		{Ref: "NVM-K7P2Q", Component: "calc.rates", Time: at, Message: "rates source answered 503", Attributes: map[string]string{"attempt": "2"}},
		{Ref: "NVM-3HD9M", Component: "clipboard.poll", Time: at.Add(-95 * time.Minute), Message: "clipboard unavailable", Attributes: map[string]string{"panic": "true"}},
		{Ref: "NVM-WXR4T", Component: analytics.ProblemComponent, Time: at.Add(-3 * time.Hour)},
	}
}
