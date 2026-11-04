package background

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/jobs"
)

// Static is a Source over a fixed list of jobs. Its controls succeed and change nothing, so goldens and the root table
// can render the page from data alone.
type Static struct {
	Jobs []jobs.Snapshot
}

func (s *Static) Snapshot() []jobs.Snapshot { return s.Jobs }

func (s *Static) RunNow(string) error { return nil }

func (s *Static) SetEnabled(string, bool) error { return nil }

func (s *Static) ClearError(string) error { return nil }

// Fixture is the job list the goldens render: an idle host job with a run in its history, an extension job that
// failed on a wake, a job that is backing off, one that is halted, and one that runs now. now anchors the times.
func Fixture(now time.Time) *Static {
	return &Static{Jobs: fixtureJobs(now)}
}

func fixtureJobs(now time.Time) []jobs.Snapshot {
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	return []jobs.Snapshot{
		{
			ID: "clipboard.images.gc", Title: "Clipboard Image Cleanup", Owner: jobs.OwnerHost, Scope: "cache", Enabled: true,
			Status:       jobs.StatusSucceeded,
			Triggers:     []jobs.Trigger{{Kind: jobs.TriggerInterval, Every: time.Hour, Delay: 30 * time.Second}},
			Runs:         12,
			LastReason:   jobs.ReasonInterval,
			LastFinished: ago(90 * time.Minute),
			LastDuration: 12 * time.Millisecond,
			History: []jobs.HistoryEntry{{
				StartedAt: ago(90 * time.Minute), FinishedAt: ago(90 * time.Minute), Duration: 12 * time.Millisecond,
				Outcome: jobs.OutcomeSucceeded, Reason: jobs.ReasonInterval,
			}},
		},
		{
			ID: "extension.acme.feed.fetch", Title: "Fetch Feed", Owner: jobs.OwnerExtension, Scope: "acme.feed", Enabled: true,
			Status:      jobs.StatusFailed,
			Triggers:    []jobs.Trigger{{Kind: jobs.TriggerEvent, Event: jobs.EventWake, Debounce: 5 * time.Second}},
			Failures:    2,
			Consecutive: 1, LastError: "the feed is down", LastReason: "event:wake",
			LastFinished: ago(5 * time.Minute), LastDuration: time.Second,
			BackoffUntil: now.Add(30 * time.Second),
			History: []jobs.HistoryEntry{{
				StartedAt: ago(5 * time.Minute), FinishedAt: ago(5 * time.Minute), Duration: time.Second,
				Outcome: jobs.OutcomeFailed, Reason: "event:wake", Error: "the feed is down",
			}},
		},
		{
			ID: "extension.acme.notes.sync", Title: "Sync Notes", Owner: jobs.OwnerExtension, Scope: "acme.notes", Enabled: true,
			Status:   jobs.StatusDisabled,
			Halted:   "Stopped after 3 failures within 1m0s. Last error: timeout",
			Triggers: []jobs.Trigger{{Kind: jobs.TriggerInterval, Every: 15 * time.Minute}},
			Failures: 3, Consecutive: 3, LastError: "timeout", LastReason: jobs.ReasonInterval,
			LastFinished: ago(20 * time.Minute),
		},
		{
			ID: "extension.acme.tasks.refresh", Title: "Refresh Tasks", Owner: jobs.OwnerExtension, Scope: "acme.tasks", Enabled: false,
			Status: jobs.StatusDisabled,
		},
		{
			ID: "extensions.triggers", Title: "Extension Triggers", Owner: jobs.OwnerHost, Scope: "extensions", Enabled: true,
			Status:       jobs.StatusRunning,
			Running:      true,
			Triggers:     []jobs.Trigger{{Kind: jobs.TriggerInterval, Every: 15 * time.Second}},
			Runs:         40,
			LastReason:   jobs.ReasonInterval,
			LastStarted:  now,
			LastFinished: ago(15 * time.Second),
		},
	}
}
