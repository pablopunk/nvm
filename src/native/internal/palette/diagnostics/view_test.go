package diagnostics

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/search"
)

// recordingHost keeps the copies a row made, so a test can read what would have reached the clipboard.
type recordingHost struct {
	copied []string
}

func (h *recordingHost) Hide()             {}
func (h *recordingHost) Pop()              {}
func (h *recordingHost) Push(view any)     {}
func (h *recordingHost) Toast(msg string)  {}
func (h *recordingHost) Refresh()          {}
func (h *recordingHost) Query() string     { return "" }
func (h *recordingHost) SetQuery(q string) {}
func (h *recordingHost) copyFn(_ search.Host, text string) {
	h.copied = append(h.copied, text)
}

// titled finds the row with the title, failing the test when it is missing.
func titled(t *testing.T, items []search.Item, title string) search.Item {
	t.Helper()
	for _, item := range items {
		if item.Title == title {
			return item
		}
	}
	t.Fatalf("no row %q", title)
	return search.Item{}
}

func TestRowsListEveryRecordNewestFirstWithItsReportingLabel(t *testing.T) {
	v := View(Static{Items: fixtureRecords()}, func() analytics.Reporting { return analytics.Requested }, (&recordingHost{}).copyFn)
	items := v.Search("")
	if len(items) != 3 {
		t.Fatalf("rows = %d, want 3", len(items))
	}
	if items[0].ID != "NVM-K7P2Q" || items[2].Title != "Problem report" {
		t.Fatalf("order or titles wrong: %q, %q", items[0].ID, items[2].Title)
	}
	if items[0].Accessory != "Reporting requested · remote availability unknown" {
		t.Fatalf("accessory = %q", items[0].Accessory)
	}
}

func TestCopyActionsCopyTheReferenceAndTheSafeDiagnostics(t *testing.T) {
	host := &recordingHost{}
	v := View(Static{Items: fixtureRecords()}, func() analytics.Reporting { return analytics.LocalOnly }, host.copyFn)
	row := titled(t, v.Search(""), "calc.rates")
	row.Actions[0].Run(host)
	row.Actions[1].Run(host)
	if len(host.copied) != 2 || host.copied[0] != "NVM-K7P2Q" {
		t.Fatalf("copied %q", host.copied)
	}
	if !strings.Contains(host.copied[1], "Reference: NVM-K7P2Q") || !strings.Contains(host.copied[1], "Error: rates source answered 503") {
		t.Fatalf("safe diagnostics = %q", host.copied[1])
	}
}

func TestSearchKeepsRowsMatchingTheTitleSubtitleOrReference(t *testing.T) {
	v := View(Static{Items: fixtureRecords()}, func() analytics.Reporting { return analytics.LocalOnly }, (&recordingHost{}).copyFn)
	if got := v.Search("clipboard"); len(got) != 1 || got[0].ID != "NVM-3HD9M" {
		t.Fatalf("search clipboard = %v", got)
	}
	if got := v.Search("wxr4t"); len(got) != 1 || got[0].Title != "Problem report" {
		t.Fatalf("search by reference = %v", got)
	}
	if got := v.Search("no such thing"); len(got) != 0 {
		t.Fatalf("search matched %d rows", len(got))
	}
}

func TestEmptyRecorderShowsNoRows(t *testing.T) {
	v := View(Static{}, func() analytics.Reporting { return analytics.Off }, (&recordingHost{}).copyFn)
	if got := v.Search(""); len(got) != 0 {
		t.Fatalf("rows = %d, want none", len(got))
	}
	if v.EmptyTitle != "No recent errors recorded" {
		t.Fatalf("empty title = %q", v.EmptyTitle)
	}
}

func TestReportingFollowsTheStateTheViewIsGiven(t *testing.T) {
	state := analytics.Requested
	v := View(Static{Items: fixtureRecords()}, func() analytics.Reporting { return state }, (&recordingHost{}).copyFn)
	if got := v.Search("")[0].Accessory; got != ReportingLabel(analytics.Requested) {
		t.Fatalf("accessory before change = %q", got)
	}
	state = analytics.Off
	if got := v.Search("")[0].Accessory; got != "Not reported" {
		t.Fatalf("accessory after consent was withdrawn = %q", got)
	}
}

func TestStaticRecorderExportsOnlyKnownReferences(t *testing.T) {
	s := Static{Items: []diag.Record{{Ref: "NVM-AAAAA", Component: "x.y"}}}
	if s.Export("NVM-AAAAA") == "" || s.Export("NVM-BBBBB") != "" {
		t.Fatal("Export did not match the reference")
	}
}

// Root has nothing to leave in this test host.
func (h *recordingHost) Root() {}
