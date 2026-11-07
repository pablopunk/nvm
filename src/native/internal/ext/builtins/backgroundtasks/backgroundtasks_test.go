package backgroundtasks

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var jobNow = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// fakeJobs holds the jobs the host would list, and the controls the built-in ran.
type fakeJobs struct {
	api.Jobs
	jobs    []api.Job
	refusal error
	ran     []string
	enabled map[string]bool
	cleared []string
}

func (f *fakeJobs) List() []api.Job { return f.jobs }

func (f *fakeJobs) Run(id string) error {
	f.ran = append(f.ran, id)
	return f.refusal
}

func (f *fakeJobs) SetEnabled(id string, enabled bool) error {
	if f.enabled == nil {
		f.enabled = map[string]bool{}
	}
	f.enabled[id] = enabled
	return f.refusal
}

func (f *fakeJobs) ClearError(id string) error {
	f.cleared = append(f.cleared, id)
	return f.refusal
}

// fakeContext is the context the built-in runs with, over one fake set of jobs.
type fakeContext struct {
	api.Context
	jobs *fakeJobs
}

func (c fakeContext) Jobs() api.Jobs { return c.jobs }

func fixtureJobs() *fakeJobs {
	return &fakeJobs{jobs: []api.Job{
		{
			ID: "clipboard.images.gc", Title: "Clipboard Image Cleanup", Owner: api.JobOwnerHost, Enabled: true,
			Status: api.JobStatusSucceeded, Triggers: []api.JobTrigger{{Kind: api.JobTriggerInterval, EveryMs: 3600000}},
			LastFinishedAt: jobNow.Add(-90 * time.Minute),
		},
		{
			ID: "extension.acme.feed.fetch", Title: "Fetch Feed", Owner: api.JobOwnerExtension, Enabled: true,
			Status: api.JobStatusFailed, LastError: "the feed is down", Triggers: []api.JobTrigger{{Kind: api.JobTriggerEvent, Event: "wake"}},
			LastFinishedAt: jobNow.Add(-5 * time.Minute), Failures: 2, ConsecutiveFailures: 1,
			History: []api.JobRun{{StartedAt: jobNow, FinishedAt: jobNow, DurationMs: 1000, Outcome: api.JobOutcomeFailed, Reason: "event:wake", Error: "the feed is down"}},
		},
		{
			ID: "extension.acme.notes.sync", Title: "Sync Notes", Owner: api.JobOwnerExtension, Enabled: true,
			Status: api.JobStatusDisabled, Halted: "Stopped after 3 failures", Triggers: []api.JobTrigger{{Kind: api.JobTriggerInterval, EveryMs: 900000}},
		},
		{ID: "extension.acme.tasks.refresh", Title: "Refresh Tasks", Owner: api.JobOwnerExtension, Status: api.JobStatusDisabled},
	}}
}

func runOpen(t *testing.T, jobs *fakeJobs) api.View {
	t.Helper()
	result, err := New().RunAction(fakeContext{jobs: jobs}, api.Invocation{Ref: api.ActionRef{ID: openAction}})
	if err != nil || result.View == nil {
		t.Fatalf("open: %v, %+v", err, result)
	}
	return *result.View
}

func act(t *testing.T, jobs *fakeJobs, ref string, args map[string]string) api.Result {
	t.Helper()
	result, err := New().RunAction(fakeContext{jobs: jobs}, api.Invocation{Ref: api.ActionRef{ID: ref, Args: args}})
	if err != nil {
		t.Fatalf("%s: %v", ref, err)
	}
	return result
}

func TestPageListsEveryJobWithItsStatusTriggersAndLastRun(t *testing.T) {
	view := runOpen(t, fixtureJobs())
	if view.Title != "Background Tasks" || view.Placeholder != "Search background tasks" || view.Refresh == nil {
		t.Fatalf("page = %q, %q", view.Title, view.Placeholder)
	}
	rows := view.Items
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	if got := rows[0].Subtitle; !strings.HasPrefix(got, "succeeded · Every 3600s · Last: ") {
		t.Fatalf("clean run subtitle = %q", got)
	}
	if got := rows[1].Subtitle; !strings.HasPrefix(got, "Failed: the feed is down · On wake · Last: ") {
		t.Fatalf("failure subtitle = %q", got)
	}
	if got := rows[2].Subtitle; !strings.HasPrefix(got, "Halted: Stopped after 3 failures · Every 900s · Last: Never") {
		t.Fatalf("halted subtitle = %q", got)
	}
	if got := rows[3].Subtitle; !strings.HasPrefix(got, "disabled · Manual · Last: Never") {
		t.Fatalf("manual subtitle = %q", got)
	}
	if rows[1].Icon != "circle-alert" || rows[0].Icon != "check" || rows[3].Icon != "power" {
		t.Fatalf("icons = %q, %q, %q", rows[1].Icon, rows[0].Icon, rows[3].Icon)
	}
	if rows[1].Accessories[0].Text != "Extension" || rows[0].Accessories[0].Text != "Host" {
		t.Fatalf("accessories = %+v", rows[1].Accessories)
	}
}

func TestRowsOpenTheirDetailsAndOfferTheControls(t *testing.T) {
	view := runOpen(t, fixtureJobs())
	failed := view.Items[1]
	if failed.Primary == nil || failed.Primary.Kind != api.ActionPushView || failed.Primary.View == nil {
		t.Fatalf("primary = %+v", failed.Primary)
	}
	if failed.Primary.View.Title != "Fetch Feed" {
		t.Fatalf("details title = %q", failed.Primary.View.Title)
	}
	labels := actionTitles(failed.Actions)
	if strings.Join(labels, "|") != "Show Details|Run Now|Disable Job|Clear Error" {
		t.Fatalf("failed actions = %v", labels)
	}
	if got := actionTitles(view.Items[2].Actions); strings.Join(got, "|") != "Show Details|Run Now|Enable Job" {
		t.Fatalf("halted actions = %v", got)
	}
}

func actionTitles(actions []api.Action) []string {
	titles := make([]string, 0, len(actions))
	for _, action := range actions {
		titles = append(titles, action.Title)
	}
	return titles
}

func TestDetailsShowTheJobAndItsRecentRuns(t *testing.T) {
	markdown := jobDetailsMarkdown(fixtureJobs().jobs[1])
	for _, want := range []string{"# Fetch Feed", "- ID: extension.acme.feed.fetch", "## Last error", "the feed is down", "## Recent runs", "event:wake · 1000ms · the feed is down"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("details lack %q", want)
		}
	}
	if !strings.Contains(jobDetailsMarkdown(fixtureJobs().jobs[3]), "_No runs recorded yet._") {
		t.Fatal("a job with no runs says so")
	}
}

func TestRunNowReplacesThePageAndSaysWhichJobRan(t *testing.T) {
	jobs := fixtureJobs()
	result := act(t, jobs, runAction, map[string]string{argID: "extension.acme.feed.fetch"})
	if len(jobs.ran) != 1 || jobs.ran[0] != "extension.acme.feed.fetch" {
		t.Fatalf("ran = %v", jobs.ran)
	}
	if result.Toast == nil || result.Toast.Message != "Ran Fetch Feed" || result.Navigation != api.NavReplace || result.View == nil {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunNowFailureShowsItsReasonAsAPage(t *testing.T) {
	jobs := fixtureJobs()
	jobs.refusal = errors.New("the job is disabled")
	result := act(t, jobs, runAction, map[string]string{argID: "extension.acme.feed.fetch"})
	if result.View == nil || result.View.Title != "Fetch Feed Failed" || result.View.Kind != api.ViewPreview {
		t.Fatalf("result = %+v", result)
	}
}

func TestEnableAndDisableReportTheNewState(t *testing.T) {
	jobs := fixtureJobs()
	disabled := act(t, jobs, toggleAction, map[string]string{argID: "extension.acme.feed.fetch", argEnabled: "false"})
	enabled := act(t, jobs, toggleAction, map[string]string{argID: "extension.acme.feed.fetch", argEnabled: "true"})
	if disabled.Toast.Message != "Disabled Fetch Feed" || enabled.Toast.Message != "Enabled Fetch Feed" {
		t.Fatalf("toasts = %q, %q", disabled.Toast.Message, enabled.Toast.Message)
	}
	if jobs.enabled["extension.acme.feed.fetch"] != true {
		t.Fatalf("enabled = %v", jobs.enabled)
	}
}

func TestClearErrorReplacesThePage(t *testing.T) {
	jobs := fixtureJobs()
	result := act(t, jobs, clearAction, map[string]string{argID: "extension.acme.feed.fetch"})
	if len(jobs.cleared) != 1 || result.Toast.Message != "Cleared Fetch Feed" || result.View == nil {
		t.Fatalf("cleared = %v, result = %+v", jobs.cleared, result)
	}
}

func TestARefusedChangeIsAnErrorToast(t *testing.T) {
	jobs := fixtureJobs()
	jobs.refusal = errors.New("jobs: the scheduler is closed")
	result := act(t, jobs, toggleAction, map[string]string{argID: "extension.acme.feed.fetch", argEnabled: "true"})
	if result.Toast == nil || result.Toast.Tone != "error" || result.View != nil {
		t.Fatalf("result = %+v", result)
	}
}

func TestAnUnknownJobIsRefusedWithoutRunning(t *testing.T) {
	jobs := fixtureJobs()
	result := act(t, jobs, runAction, map[string]string{argID: "missing"})
	if len(jobs.ran) != 0 || result.Toast == nil || result.Toast.Tone != "error" {
		t.Fatalf("ran = %v, result = %+v", jobs.ran, result)
	}
}

func TestTheSchedulesAreSpelledAsTheNativePageSpelledThem(t *testing.T) {
	cases := map[string]api.JobTrigger{
		"Startup":   {Kind: api.JobTriggerStartup},
		"Every 15s": {Kind: api.JobTriggerInterval, EveryMs: 15000},
		"On wake":   {Kind: api.JobTriggerEvent, Event: "wake"},
		"Manual":    {Kind: "unknown"},
	}
	for want, trigger := range cases {
		if got := triggerSummary([]api.JobTrigger{trigger}); got != want {
			t.Fatalf("triggerSummary(%+v) = %q, want %q", trigger, got, want)
		}
	}
	if got := triggerSummary(nil); got != "Manual" {
		t.Fatalf("no triggers = %q", got)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	_, err := New().RunAction(fakeContext{jobs: fixtureJobs()}, api.Invocation{Ref: api.ActionRef{ID: "delete"}})
	if !errors.Is(err, api.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}
