package jobs

import (
	"time"
)

// timeout is the run budget of a definition, with the default for zero.
func (b Budget) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return DefaultTimeout
}

// delayAfter is the wait after the n-th failure in a row (n counts from 1): the initial delay doubled n-1 times, capped.
func (b Backoff) delayAfter(consecutive int) time.Duration {
	initial, limit := b.Initial, b.Max
	if initial <= 0 {
		initial = DefaultBackoffInitial
	}
	if limit <= 0 {
		limit = DefaultBackoffMax
	}
	delay := initial
	for step := 1; step < consecutive && delay < limit; step++ {
		delay *= 2
	}
	return min(delay, limit)
}

// limits returns the failure count and window of a halt policy, with the defaults for zero fields.
func (h HaltPolicy) limits() (int, time.Duration) {
	failures, within := h.Failures, h.Within
	if failures <= 0 {
		failures = DefaultHaltFailures
	}
	if within <= 0 {
		within = DefaultHaltWindow
	}
	return failures, within
}

// prune keeps the times that fall within window of now.
func prune(times []time.Time, now time.Time, window time.Duration) []time.Time {
	kept := times[:0]
	for _, at := range times {
		if now.Sub(at) < window {
			kept = append(kept, at)
		}
	}
	return kept
}

// debounceDelay is how long a debounced event waits from now: its quiet period, cut short so the run comes no later
// than MaxWait after the first event of the burst.
func debounceDelay(t Trigger, firstAt, now time.Time) time.Duration {
	delay := t.Debounce
	if t.MaxWait > 0 {
		delay = min(delay, firstAt.Add(t.MaxWait).Sub(now))
	}
	return max(delay, 0)
}

// missedDue reports whether an interval run fell due after the last finished run, while the app was not running. A
// missed run is caught up once, unless the trigger skips missed runs.
func missedDue(t Trigger, lastFinished, now time.Time) bool {
	if t.Missed == MissedSkip || lastFinished.IsZero() {
		return false
	}
	return !now.Before(lastFinished.Add(t.Every))
}
