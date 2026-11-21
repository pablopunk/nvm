package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// backgroundNow anchors the times the Background Tasks goldens show. It is the fixture epoch, so every relative time
// reads the same on every run.
var backgroundNow = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// fixedJobs is the background jobs with one fixed set, so a golden shows the jobs its fixture names.
type fixedJobs struct {
	jobs []api.Job
}

func (f fixedJobs) List() []api.Job               { return f.jobs }
func (f fixedJobs) Run(string) error              { return nil }
func (f fixedJobs) SetEnabled(string, bool) error { return nil }
func (f fixedJobs) ClearError(string) error       { return nil }

// backgroundGoldenJobs are one job in each state the list shows: a clean run, a failure, a halted job, a switched-off job
// and the host's own trigger loop.
func backgroundGoldenJobs(now time.Time) []api.Job {
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	return []api.Job{
		{
			ID: "clipboard.images.gc", Title: "Clipboard Image Cleanup", Owner: api.JobOwnerHost, Scope: "cache", Enabled: true,
			Status: api.JobStatusSucceeded, Runs: 12, LastReason: "interval",
			Triggers:      []api.JobTrigger{{Kind: api.JobTriggerInterval, EveryMs: 3600000}},
			LastStartedAt: ago(90 * time.Minute), LastFinishedAt: ago(90 * time.Minute), LastDurationMs: 12,
			History: []api.JobRun{{StartedAt: ago(90 * time.Minute), FinishedAt: ago(90 * time.Minute), DurationMs: 12, Outcome: api.JobOutcomeSucceeded, Reason: "interval"}},
		},
		{
			ID: "extension.acme.feed.fetch", Title: "Fetch Feed", Owner: api.JobOwnerExtension, Scope: "acme.feed", Enabled: true,
			Status: api.JobStatusFailed, Failures: 2, ConsecutiveFailures: 1, LastError: "the feed is down", LastReason: "event:wake",
			Triggers:       []api.JobTrigger{{Kind: api.JobTriggerEvent, Event: "wake"}},
			LastFinishedAt: ago(5 * time.Minute), LastDurationMs: 1000, BackoffUntil: now.Add(30 * time.Second),
			History: []api.JobRun{{StartedAt: ago(5 * time.Minute), FinishedAt: ago(5 * time.Minute), DurationMs: 1000, Outcome: api.JobOutcomeFailed, Reason: "event:wake", Error: "the feed is down"}},
		},
		{
			ID: "extension.acme.notes.sync", Title: "Sync Notes", Owner: api.JobOwnerExtension, Scope: "acme.notes", Enabled: true,
			Status: api.JobStatusDisabled, Halted: "Stopped after 3 failures within 1m0s. Last error: timeout",
			Triggers: []api.JobTrigger{{Kind: api.JobTriggerInterval, EveryMs: 900000}}, Failures: 3, ConsecutiveFailures: 3,
			LastError: "timeout", LastReason: "interval", LastFinishedAt: ago(20 * time.Minute),
		},
		{
			ID: "extension.acme.tasks.refresh", Title: "Refresh Tasks", Owner: api.JobOwnerExtension, Scope: "acme.tasks",
			Status: api.JobStatusDisabled,
		},
		{
			ID: "extensions.triggers", Title: "Extension Triggers", Owner: api.JobOwnerHost, Scope: "extensions", Enabled: true,
			Status: api.JobStatusRunning, Running: true, Runs: 40, LastReason: "interval", LastStartedAt: now, LastFinishedAt: ago(15 * time.Second),
			Triggers: []api.JobTrigger{{Kind: api.JobTriggerInterval, EveryMs: 15000}},
		},
	}
}

// TestPaletteBackgroundTasksList renders the list with one row per state, the action menu of the failed job, and the
// details of that job, which Enter opens.
func TestPaletteBackgroundTasksList(t *testing.T) {
	p := goldenPalette(t)
	p.exts.mgr.Store(newFixtureManagerWithJobs(t, p, fixedJobs{jobs: backgroundGoldenJobs(backgroundNow)}))
	goldenOpen(p, p.backgroundTasksPage())
	golden.AssertMatrix(t, "background-list", p.View, paletteGoldens)

	p = goldenPalette(t)
	p.exts.mgr.Store(newFixtureManagerWithJobs(t, p, fixedJobs{jobs: backgroundGoldenJobs(backgroundNow)}))
	goldenOpen(p, p.backgroundTasksPage())
	p.Choose(1)
	golden.AssertMatrix(t, "background-history", p.View, paletteGoldens)

	p = goldenPalette(t)
	p.exts.mgr.Store(newFixtureManagerWithJobs(t, p, fixedJobs{jobs: backgroundGoldenJobs(backgroundNow)}))
	goldenOpen(p, p.backgroundTasksPage())
	p.OpenActionMenu(1)
	golden.AssertMatrix(t, "background-actions", p.View, paletteGoldens)
}
