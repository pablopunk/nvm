package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Reasons a run starts for, as the run's Reason and the history show them.
const (
	ReasonManual     = "manual"
	ReasonStartup    = "startup"
	ReasonInterval   = "interval"
	ReasonMissed     = "missed"
	ReasonRetry      = "retry"
	reasonOverBudget = "over the run budget"
)

// start runs the job now, or queues it behind the run in progress, or defers it past a backoff, or skips it when the
// budget is spent. Manual runs ignore backoff. Interval runs that find the job busy are dropped, not queued.
func (s *Scheduler) start(id, reason string, payload Payload, manual bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	r, ok := s.jobs[id]
	if !ok {
		return ErrUnknownJob
	}
	if !r.runnable() {
		if manual {
			return ErrDisabled
		}
		return nil
	}
	now := s.clock.Now()
	switch {
	case r.running:
		s.queueLocked(r, reason, payload)
		return nil
	case !manual && now.Before(r.backoffUntil):
		s.armRetryLocked(r)
		return nil
	case s.overBudgetLocked(r, now):
		s.recordLocked(r, HistoryEntry{StartedAt: now, FinishedAt: now, Outcome: OutcomeSkipped, Reason: reason, Error: reasonOverBudget})
		s.notifyLocked()
		return nil
	}
	s.beginLocked(r, reason, payload, now)
	return nil
}

// queueLocked keeps the latest reason of a run that waits for the running one and merges the payloads of the events
// that arrive meanwhile. Interval ticks are not queued.
func (s *Scheduler) queueLocked(r *record, reason string, payload Payload) {
	if reason == ReasonInterval {
		return
	}
	var previous Payload
	if r.pending != nil {
		previous = r.pending.payload
	}
	r.pending = &queuedRun{reason: reason, payload: mergePayloads(previous, payload)}
	s.notifyLocked()
}

// beginLocked marks the job running and hands its function to a goroutine.
func (s *Scheduler) beginLocked(r *record, reason string, payload Payload, now time.Time) {
	r.running = true
	r.lastReason = reason
	r.lastStarted = now
	r.startTimes = append(prune(r.startTimes, now, r.def.Budget.Window), now)
	run := Run{ID: r.def.ID, Reason: reason, Payload: payload, StartedAt: now}
	def := r.def
	s.goGuarded("jobs.run."+def.ID, func() {
		s.execute(def, run)
	})
	s.notifyLocked()
}

// execute calls the job's function under safego with its budget and reports the outcome to finish.
func (s *Scheduler) execute(def Definition, run Run) {
	ctx := s.ctx
	finishObservation := func(RunOutcome) {}
	if s.runObserver != nil {
		observedCtx, done := s.runObserver.BeginRun(ctx)
		if observedCtx != nil {
			ctx = observedCtx
		}
		if done != nil {
			finishObservation = done
		}
	}
	var skippedOutcome *Skipped
	err := safego.Run("jobs."+def.ID, func(ctx context.Context) error {
		err := def.Run(ctx, run)
		var skipped Skipped
		if errors.As(err, &skipped) {
			skippedOutcome = &skipped
			return nil
		}
		return err
	}, s.safegoOptions(safego.WithContext(ctx), safego.WithClock(s.clock), safego.WithTimeout(def.Budget.timeout()))...)
	if err == nil && skippedOutcome != nil {
		err = *skippedOutcome
	}
	finishObservation(observedOutcome(err))
	s.finish(def.ID, run, err)
}

func observedOutcome(err error) RunOutcome {
	var skipped Skipped
	if errors.As(err, &skipped) {
		return RunSkipped
	}
	if err == nil {
		return RunSucceeded
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RunTimedOut
	}
	if errors.Is(err, context.Canceled) {
		return RunCancelled
	}
	return RunFailed
}

// finish records the outcome of a run, halts the job when its policy says so, and starts the run queued behind it.
func (s *Scheduler) finish(id string, run Run, err error) {
	s.mu.Lock()
	r, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	now := s.clock.Now()
	entry := HistoryEntry{StartedAt: run.StartedAt, FinishedAt: now, Duration: now.Sub(run.StartedAt), Reason: run.Reason, Outcome: OutcomeSucceeded}
	r.running = false
	var skipped Skipped
	if errors.As(err, &skipped) {
		entry.Outcome, entry.Error = OutcomeSkipped, skipped.Reason
		if count := len(r.startTimes); count > 0 && r.startTimes[count-1].Equal(run.StartedAt) {
			r.startTimes = r.startTimes[:count-1]
		}
	} else if err != nil {
		entry.Outcome, entry.Error = OutcomeFailed, err.Error()
		s.noteFailureLocked(r, now, err)
	} else {
		noteSuccessLocked(r)
	}
	if entry.Outcome != OutcomeSkipped {
		r.lastFinished, r.lastDuration = now, entry.Duration
	}
	s.recordLocked(r, entry)
	if entry.Outcome == OutcomeSkipped {
		r.restoreLastRun()
	}
	queued := r.pending
	r.pending = nil
	halted := !r.runnable()
	if halted {
		s.stopLocked(r)
	}
	s.notifyLocked()
	s.mu.Unlock()
	if queued != nil && !halted {
		_ = s.start(id, queued.reason, queued.payload, false)
	}
}

// noteFailureLocked counts a failed run, starts its backoff and halts the job when the failures in the halt window
// reach the limit.
func (s *Scheduler) noteFailureLocked(r *record, now time.Time, err error) {
	failures, within := r.def.Halt.limits()
	r.failures++
	r.consecutive++
	r.lastOutcome, r.lastError = OutcomeFailed, err.Error()
	r.backoffUntil = now.Add(r.def.Backoff.delayAfter(r.consecutive))
	r.failureTimes = append(prune(r.failureTimes, now, within), now)
	if len(r.failureTimes) < failures {
		return
	}
	r.halted = fmt.Sprintf("Stopped after %d failures within %s. Last error: %s", len(r.failureTimes), within, err)
	s.log.Warn("jobs.halted", logx.F{"job": r.def.ID, "failures": len(r.failureTimes)})
}

// noteSuccessLocked counts a successful run and clears the failure streak.
func noteSuccessLocked(r *record) {
	r.runs++
	r.consecutive = 0
	r.backoffUntil = time.Time{}
	r.lastOutcome, r.lastError = OutcomeSucceeded, ""
}

// recordLocked puts an entry at the top of the job's history and saves the job.
func (s *Scheduler) recordLocked(r *record, entry HistoryEntry) {
	history := append([]HistoryEntry{entry}, r.history...)
	r.history = history[:min(len(history), maxHistory)]
	s.saveLocked(r)
}

// overBudgetLocked reports whether the job already started its budgeted number of runs in the window.
func (s *Scheduler) overBudgetLocked(r *record, now time.Time) bool {
	budget := r.def.Budget
	if budget.MaxRuns <= 0 {
		return false
	}
	r.startTimes = prune(r.startTimes, now, budget.Window)
	return len(r.startTimes) >= budget.MaxRuns
}

// armRetryLocked starts one timer that runs the job when its backoff ends. Only one is armed at a time.
func (s *Scheduler) armRetryLocked(r *record) {
	if r.retryArmed || r.stop == nil {
		return
	}
	r.retryArmed = true
	id, stop := r.def.ID, r.stop
	timer := s.clock.NewTicker(r.backoffUntil.Sub(s.clock.Now()))
	s.notifyLocked()
	s.goGuarded("jobs.retry."+id, func() {
		ok := s.await(timer, stop)
		s.mu.Lock()
		if current, found := s.jobs[id]; found {
			current.retryArmed = false
		}
		s.mu.Unlock()
		if ok {
			_ = s.start(id, ReasonRetry, nil, false)
		}
	})
}
