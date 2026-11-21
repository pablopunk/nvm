package api

import (
	"errors"
	"time"
)

// MaxScheduledRuns is how many one-shot runs an extension may hold at once. Scheduling past it is refused.
const MaxScheduledRuns = 16

// ScheduleKeyLimit is the longest key a one-shot run may be scheduled under.
const ScheduleKeyLimit = 64

// Scheduler is the one-shot trigger: it runs one of the extension's own handlers at a time, once, and can be cancelled
// before that. The host owns the timing and runs the handler as it runs any action, with Launch.Reason "once" and
// Launch.Event "once". A run is kept only while the app runs: a run the app quits before is not run at the next start,
// and a run is dropped when the extension is reloaded or disabled.
type Scheduler interface {
	// Once schedules ref's handler for at, under key. A run already scheduled under key is replaced. A time already past
	// runs at once. Keys are at most ScheduleKeyLimit bytes, and at most MaxScheduledRuns runs may be pending.
	Once(key string, at time.Time, ref ActionRef) error
	// Cancel drops the run scheduled under key and reports whether one was pending.
	Cancel(key string) (bool, error)
	// Pending reports the time of the run scheduled under key, and whether one is pending.
	Pending(key string) (time.Time, bool, error)
}

// ValidateScheduleKey checks the key of a one-shot run: it is named, and short enough for the host to keep.
func ValidateScheduleKey(key string) error {
	if key == "" {
		return errors.New("a scheduled run needs a key")
	}
	if len(key) > ScheduleKeyLimit {
		return errors.New("a scheduled run's key is longer than 64 bytes")
	}
	return nil
}
