package jobs

import (
	"slices"
	"time"
)

// queuedRun is the run that waits for the running one to finish: the latest reason and the merged payload.
type queuedRun struct {
	reason  string
	payload Payload
}

// pendingEvent is a debounced event that has not fired yet. Closing stop cancels its timer.
type pendingEvent struct {
	firstAt time.Time
	stop    chan struct{}
	payload Payload
}

// record is the scheduler's state for one job. Every field is guarded by the scheduler's lock.
type record struct {
	def         Definition
	userEnabled *bool
	halted      string
	running     bool
	pending     *queuedRun
	stop        chan struct{}
	events      map[int]*pendingEvent
	retryArmed  bool
	nextAt      time.Time

	backoffUntil time.Time
	startTimes   []time.Time
	failureTimes []time.Time
	consecutive  int

	runs, failures int
	lastReason     string
	lastStarted    time.Time
	lastFinished   time.Time
	lastDuration   time.Duration
	lastOutcome    Outcome
	lastError      string
	history        []HistoryEntry
}

// newRecord restores a job from its saved state. The last run is read back from the history, so the job shows what
// it did before the restart.
func newRecord(def Definition, saved JobState) *record {
	r := &record{
		def:         def,
		userEnabled: saved.Enabled,
		halted:      saved.Halted,
		runs:        saved.Runs,
		failures:    saved.Failures,
		events:      map[int]*pendingEvent{},
		history:     slices.Clone(saved.History),
	}
	r.history = r.history[:min(len(r.history), maxHistory)]
	r.restoreLastRun()
	return r
}

// restoreLastRun copies the newest run that executed (skipped runs never did) into the last-run fields.
func (r *record) restoreLastRun() {
	for _, entry := range r.history {
		if entry.Outcome == OutcomeSkipped {
			continue
		}
		r.lastReason, r.lastStarted, r.lastFinished = entry.Reason, entry.StartedAt, entry.FinishedAt
		r.lastDuration, r.lastOutcome, r.lastError = entry.Duration, entry.Outcome, entry.Error
		return
	}
}

// enabled is the user's choice, or the definition's default when the user has not chosen.
func (r *record) enabled() bool {
	if r.userEnabled != nil {
		return *r.userEnabled
	}
	return !r.def.StartDisabled
}

// runnable reports whether triggers and runs may happen: enabled and not halted by a failure policy.
func (r *record) runnable() bool {
	return r.enabled() && r.halted == ""
}

// status is what the palette shows for the job at now.
func (r *record) status(now time.Time) Status {
	switch {
	case !r.runnable():
		return StatusDisabled
	case r.running:
		return StatusRunning
	case r.retryArmed && now.Before(r.backoffUntil):
		return StatusBackingOff
	case r.lastOutcome == OutcomeFailed:
		return StatusFailed
	case r.lastOutcome == OutcomeSucceeded:
		return StatusSucceeded
	default:
		return StatusIdle
	}
}

// snapshot copies the job's state for a view.
func (r *record) snapshot(now time.Time) Snapshot {
	owner := r.def.Owner
	if owner == "" {
		owner = OwnerHost
	}
	return Snapshot{
		ID:           r.def.ID,
		Title:        r.def.Title,
		Owner:        owner,
		Scope:        r.def.Scope,
		Status:       r.status(now),
		Enabled:      r.enabled(),
		Halted:       r.halted,
		Running:      r.running,
		Triggers:     slices.Clone(r.def.Triggers),
		Runs:         r.runs,
		Failures:     r.failures,
		Consecutive:  r.consecutive,
		LastReason:   r.lastReason,
		LastStarted:  r.lastStarted,
		LastFinished: r.lastFinished,
		LastDuration: r.lastDuration,
		LastError:    r.lastError,
		NextRunAt:    r.nextAt,
		BackoffUntil: r.backoffUntil,
		History:      slices.Clone(r.history),
	}
}

// persisted is the part of the job that survives a restart.
func (r *record) persisted() JobState {
	return JobState{
		Enabled:  r.userEnabled,
		Halted:   r.halted,
		Runs:     r.runs,
		Failures: r.failures,
		History:  slices.Clone(r.history),
	}
}
