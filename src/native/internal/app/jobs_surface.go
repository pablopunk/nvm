package app

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
)

// errJobsNotStarted refuses a change to a job while the scheduler is not running yet.
var errJobsNotStarted = errors.New("background jobs are not running yet")

// jobsSurface serves the background jobs to the extension API: the scheduler's jobs in the API's shape, and its controls.
type jobsSurface struct {
	p *App
}

// List is every job the scheduler holds, in the order it keeps them. Before the scheduler starts there are none.
func (s jobsSurface) List() []api.Job {
	if s.p.background == nil {
		return nil
	}
	snapshots := s.p.background.Snapshot()
	out := make([]api.Job, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, apiJob(snapshot))
	}
	return out
}

// Run starts a job now, outside its triggers.
func (s jobsSurface) Run(id string) error {
	if s.p.background == nil {
		return errJobsNotStarted
	}
	return s.p.background.RunNow(id)
}

// SetEnabled turns a job on or off.
func (s jobsSurface) SetEnabled(id string, enabled bool) error {
	if s.p.background == nil {
		return errJobsNotStarted
	}
	return s.p.background.SetEnabled(id, enabled)
}

// ClearError forgets the last error of a job.
func (s jobsSurface) ClearError(id string) error {
	if s.p.background == nil {
		return errJobsNotStarted
	}
	return s.p.background.ClearError(id)
}

// apiJob converts one job's snapshot for the extension API.
func apiJob(snap jobs.Snapshot) api.Job {
	return api.Job{
		ID: snap.ID, Title: snap.Title, Owner: string(snap.Owner), Scope: snap.Scope,
		Status: string(snap.Status), Enabled: snap.Enabled, Halted: snap.Halted, Running: snap.Running,
		Triggers: apiJobTriggers(snap.Triggers), Runs: snap.Runs, Failures: snap.Failures,
		ConsecutiveFailures: snap.Consecutive, LastReason: snap.LastReason,
		LastStartedAt: snap.LastStarted, LastFinishedAt: snap.LastFinished, LastDurationMs: snap.LastDuration.Milliseconds(),
		LastError: snap.LastError, NextRunAt: snap.NextRunAt, BackoffUntil: snap.BackoffUntil,
		History: apiJobRuns(snap.History),
	}
}

// apiJobTriggers converts a job's triggers. An interval reads as milliseconds; the other kinds carry their event.
func apiJobTriggers(triggers []jobs.Trigger) []api.JobTrigger {
	out := make([]api.JobTrigger, 0, len(triggers))
	for _, trigger := range triggers {
		out = append(out, api.JobTrigger{Kind: string(trigger.Kind), EveryMs: trigger.Every.Milliseconds(), Event: trigger.Event})
	}
	return out
}

// apiJobRuns converts a job's run history, in the order the scheduler keeps it.
func apiJobRuns(history []jobs.HistoryEntry) []api.JobRun {
	out := make([]api.JobRun, 0, len(history))
	for _, entry := range history {
		out = append(out, api.JobRun{
			StartedAt: entry.StartedAt, FinishedAt: entry.FinishedAt, DurationMs: entry.Duration.Milliseconds(),
			Outcome: string(entry.Outcome), Reason: entry.Reason, Error: entry.Error,
		})
	}
	return out
}
