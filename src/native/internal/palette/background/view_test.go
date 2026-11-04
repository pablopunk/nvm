package background

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeSource keeps the jobs the page shows and records the controls it was asked to use.
type fakeSource struct {
	jobs     []jobs.Snapshot
	runErr   error
	ran      []string
	enabled  map[string]bool
	cleared  []string
}

func (f *fakeSource) Snapshot() []jobs.Snapshot { return f.jobs }

func (f *fakeSource) RunNow(id string) error {
	f.ran = append(f.ran, id)
	return f.runErr
}

func (f *fakeSource) SetEnabled(id string, enabled bool) error {
	if f.enabled == nil {
		f.enabled = map[string]bool{}
	}
	f.enabled[id] = enabled
	return nil
}

func (f *fakeSource) ClearError(id string) error {
	f.cleared = append(f.cleared, id)
	return nil
}

// recordingHost keeps what an action told the palette to do.
type recordingHost struct {
	toasts    []string
	refreshes int
	pushed    []any
}

func (h *recordingHost) Hide()             {}
func (h *recordingHost) Pop()              {}
func (h *recordingHost) Root()             {}
func (h *recordingHost) Push(view any)     { h.pushed = append(h.pushed, view) }
func (h *recordingHost) Toast(msg string)  { h.toasts = append(h.toasts, msg) }
func (h *recordingHost) Refresh()          { h.refreshes++ }
func (h *recordingHost) Query() string     { return "" }
func (h *recordingHost) SetQuery(q string) {}

var epoch = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

func testJobs() []jobs.Snapshot {
	return Fixture(epoch).Jobs
}

func rowTitled(t *testing.T, items []search.Item, title string) search.Item {
	t.Helper()
	for _, item := range items {
		if item.Title == title {
			return item
		}
	}
	t.Fatalf("no row %q in %d rows", title, len(items))
	return search.Item{}
}

func actionTitled(t *testing.T, item search.Item, title string) search.Action {
	t.Helper()
	for _, action := range item.Actions {
		if action.Title == title {
			return action
		}
	}
	t.Fatalf("no action %q on %s; have %v", title, item.Title, actionTitles(item))
	return search.Action{}
}

func actionTitles(item search.Item) []string {
	var titles []string
	for _, action := range item.Actions {
		titles = append(titles, action.Title)
	}
	return titles
}

func TestRowsShowStatusTriggersAndLastRun(t *testing.T) {
	v := View(&fakeSource{jobs: testJobs()}, clockAt(epoch))
	items := v.Search("")
	if len(items) != 5 {
		t.Fatalf("rows = %d", len(items))
	}
	cleanup := rowTitled(t, items, "Clipboard Image Cleanup")
	if cleanup.Subtitle != "Succeeded · Every 1h · Last 1 h ago" || cleanup.Accessory != "Host" {
		t.Fatalf("cleanup row = %q / %q", cleanup.Subtitle, cleanup.Accessory)
	}
	fetch := rowTitled(t, items, "Fetch Feed")
	if fetch.Subtitle != "Failed: the feed is down · On wake · Last 5 min ago" || fetch.Accessory != "Extension" {
		t.Fatalf("fetch row = %q / %q", fetch.Subtitle, fetch.Accessory)
	}
	off := rowTitled(t, items, "Sync Notes")
	if !strings.HasPrefix(off.Subtitle, "Halted: Stopped after 3 failures") || off.Subtitle == "" {
		t.Fatalf("halted row = %q", off.Subtitle)
	}
}

func TestEnterOpensTheRunHistoryOfTheJob(t *testing.T) {
	source := &fakeSource{jobs: testJobs()}
	host := &recordingHost{}
	item := rowTitled(t, View(source, clockAt(epoch)).Search(""), "Fetch Feed")
	item.Actions[0].Run(host)
	if len(host.pushed) != 1 {
		t.Fatalf("pushed %d views", len(host.pushed))
	}
	history := host.pushed[0].(*palette.View)
	runs := history.Search("")
	wantWhen := epoch.Add(-5 * time.Minute).Local().Format("2006-01-02 15:04:05")
	if len(runs) != 1 || runs[0].Title != "Failed · event:wake" || runs[0].Subtitle != wantWhen+" · the feed is down" {
		t.Fatalf("history rows = %+v", runs)
	}
}

func TestRunNowRunsTheJobAndRefreshes(t *testing.T) {
	source := &fakeSource{jobs: testJobs()}
	host := &recordingHost{}
	item := rowTitled(t, View(source, clockAt(epoch)).Search(""), "Clipboard Image Cleanup")
	actionTitled(t, item, "Run Now").Run(host)
	if len(source.ran) != 1 || source.ran[0] != "clipboard.images.gc" || host.refreshes != 1 || host.toasts[0] != "Ran Clipboard Image Cleanup" {
		t.Fatalf("ran %v, refreshes %d, toasts %v", source.ran, host.refreshes, host.toasts)
	}
}

func TestRunNowShowsTheRefusalAsAToast(t *testing.T) {
	source := &fakeSource{jobs: testJobs(), runErr: errors.New("jobs: the job is disabled")}
	host := &recordingHost{}
	item := rowTitled(t, View(source, clockAt(epoch)).Search(""), "Sync Notes")
	actionTitled(t, item, "Run Now").Run(host)
	if host.refreshes != 0 || host.toasts[0] != "jobs: the job is disabled" {
		t.Fatalf("refreshes %d, toasts %v", host.refreshes, host.toasts)
	}
}

func TestEnableAndDisableFlipTheJob(t *testing.T) {
	source := &fakeSource{jobs: testJobs()}
	host := &recordingHost{}
	items := View(source, clockAt(epoch)).Search("")
	actionTitled(t, rowTitled(t, items, "Clipboard Image Cleanup"), "Disable Job").Run(host)
	actionTitled(t, rowTitled(t, items, "Sync Notes"), "Enable Job").Run(host)
	if source.enabled["clipboard.images.gc"] || !source.enabled["extension.acme.notes.sync"] {
		t.Fatalf("enabled = %v", source.enabled)
	}
}

func TestClearErrorOnlyAppearsForAJobWithAnError(t *testing.T) {
	source := &fakeSource{jobs: testJobs()}
	host := &recordingHost{}
	items := View(source, clockAt(epoch)).Search("")
	actionTitled(t, rowTitled(t, items, "Fetch Feed"), "Clear Error").Run(host)
	if len(source.cleared) != 1 || source.cleared[0] != "extension.acme.feed.fetch" {
		t.Fatalf("cleared = %v", source.cleared)
	}
	for _, action := range rowTitled(t, items, "Clipboard Image Cleanup").Actions {
		if action.Title == "Clear Error" {
			t.Fatal("a job without an error offers Clear Error")
		}
	}
}

func TestSearchMatchesTitleAndStatusText(t *testing.T) {
	v := View(&fakeSource{jobs: testJobs()}, clockAt(epoch))
	if got := v.Search("feed"); len(got) != 1 || got[0].Title != "Fetch Feed" {
		t.Fatalf("search feed = %v", got)
	}
	if got := v.Search("halted"); len(got) != 1 || got[0].Title != "Sync Notes" {
		t.Fatalf("search halted = %v", got)
	}
	if got := v.Search("nothing like this"); len(got) != 0 {
		t.Fatalf("search matched %d rows", len(got))
	}
}

func TestNilSourceShowsNoRows(t *testing.T) {
	if got := View(nil, clockAt(epoch)).Search(""); len(got) != 0 {
		t.Fatalf("rows = %d", len(got))
	}
}

func TestDurationsAreSpelledInTheLargestWholeUnit(t *testing.T) {
	cases := map[time.Duration]string{time.Hour: "1h", 90 * time.Minute: "90m", 45 * time.Second: "45s", 2 * time.Hour: "2h"}
	for d, want := range cases {
		if got := duration(d); got != want {
			t.Errorf("duration(%v) = %q, want %q", d, got, want)
		}
	}
}

func clockAt(t time.Time) func() time.Time {
	return func() time.Time { return t }
}
