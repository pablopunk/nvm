package services

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// schedulesPerMinute bounds how often an extension may schedule a run, so a loop cannot churn the pending runs.
const schedulesPerMinute = 60

// scheduledRun is one pending run: when it is due, and the channel that cancels it.
type scheduledRun struct {
	at   time.Time
	stop chan struct{}
}

// scheduleBook holds one extension's pending one-shot runs. A run leaves the book when it fires, when it is cancelled
// and when the book closes; the handler closes it when the extension is reloaded or disabled.
type scheduleBook struct {
	mu     sync.Mutex
	clock  platform.Clock
	fire   func(api.Invocation)
	limit  *rateLimiter
	runs   map[string]*scheduledRun
	closed bool
}

func newScheduleBook(clock platform.Clock, fire func(api.Invocation)) *scheduleBook {
	return &scheduleBook{clock: clock, fire: fire, limit: newRateLimiter(clock, schedulesPerMinute), runs: map[string]*scheduledRun{}}
}

// serveScheduler answers the one-shot trigger service. Scheduling needs no declaration: a run is the extension's own
// handler, and the rate limit and the pending limit bound what it may do.
func (h *Handler) serveScheduler(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpOnce:
		var args protocol.OnceArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		return nil, h.schedules.once(args.Key, args.At, args.Ref)
	case protocol.OpCancel:
		var args protocol.ScheduleKeyArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		return h.schedules.cancel(args.Key), nil
	case protocol.OpPending:
		var args protocol.ScheduleKeyArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		at, pending := h.schedules.pending(args.Key)
		return protocol.PendingReply{At: at, Pending: pending}, nil
	}
	return nil, invalid("scheduler has no operation %q", call.Op)
}

// once schedules ref under key for at. A run already pending under key is replaced, as a timer replaces the timer
// before it. The run waits on the clock in its own goroutine, so a run that is due fires without a call from the child.
func (b *scheduleBook) once(key string, at time.Time, ref api.ActionRef) error {
	if err := api.ValidateScheduleKey(key); err != nil {
		return invalid("%s", err)
	}
	if ref.ID == "" {
		return invalid("a scheduled run needs an action to run")
	}
	if !b.limit.allow() {
		return rateExceeded("scheduled runs")
	}
	run, err := b.add(key, at)
	if err != nil {
		return err
	}
	delay := at.Sub(b.clock.Now())
	safego.Go("services.schedule", func(context.Context) { b.wait(key, run, ref, delay) })
	return nil
}

// add records a pending run under key, replacing the one there, and refuses a new key past the pending limit.
func (b *scheduleBook) add(key string, at time.Time) (*scheduledRun, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, unavailable("the extension is no longer running")
	}
	if _, replaces := b.runs[key]; !replaces && len(b.runs) >= api.MaxScheduledRuns {
		return nil, invalid("an extension may hold %d scheduled runs", api.MaxScheduledRuns)
	}
	if old, ok := b.runs[key]; ok {
		close(old.stop)
	}
	run := &scheduledRun{at: at, stop: make(chan struct{})}
	b.runs[key] = run
	return run, nil
}

// wait blocks until the run is due or cancelled. A due run that is still the book's run under key fires once.
func (b *scheduleBook) wait(key string, run *scheduledRun, ref api.ActionRef, delay time.Duration) {
	select {
	case <-b.clock.After(delay):
	case <-run.stop:
		return
	}
	if !b.take(key, run) {
		return
	}
	b.fire(api.Invocation{
		Ref:    ref,
		Launch: api.LaunchContext{Reason: "once", Event: "once", StartedAtMs: b.clock.Now().UnixMilli()},
	})
}

// take removes run from the book when it is still the run pending under key, and reports whether it was.
func (b *scheduleBook) take(key string, run *scheduledRun) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.runs[key] != run {
		return false
	}
	delete(b.runs, key)
	return true
}

// cancel drops the run pending under key and reports whether there was one.
func (b *scheduleBook) cancel(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	run, ok := b.runs[key]
	if !ok {
		return false
	}
	close(run.stop)
	delete(b.runs, key)
	return true
}

// pending reports when the run under key is due, and whether one is pending.
func (b *scheduleBook) pending(key string) (time.Time, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	run, ok := b.runs[key]
	if !ok {
		return time.Time{}, false
	}
	return run.at, true
}

// close drops every pending run and refuses new ones. It is safe to call more than once.
func (b *scheduleBook) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for key, run := range b.runs {
		close(run.stop)
		delete(b.runs, key)
	}
}
