package safego

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	backoffBase = 250 * time.Millisecond
	backoffMax  = 30 * time.Second
)

// Go starts fn on a new goroutine under the name name. fn receives a context that is cancelled when the
// parent is done or the attempt times out. A panic is recovered and reported; with WithBackoff the
// function restarts, otherwise the goroutine ends.
func Go(name string, fn func(ctx context.Context), opts ...Option) {
	o := resolve(opts)
	go track(name, func() {
		_ = supervise(name, o, func(ctx context.Context) error {
			fn(ctx)
			return nil
		})
	})
}

// Run calls fn on the calling goroutine under the same guard as Go and returns the outcome: nil, the
// error fn returned, a *PanicError, a timeout error, or an error wrapping ErrDisabled.
func Run(name string, fn func(ctx context.Context) error, opts ...Option) error {
	return supervise(name, resolve(opts), fn)
}

// supervise runs attempts until one succeeds, the parent is done, or the restart policy disables the
// call. Each failure is reported exactly once, with the disabled flag set on the last one.
func supervise(name string, o options, fn func(context.Context) error) error {
	ctx, cancel := context.WithCancel(o.parent)
	defer cancel()
	var recent []time.Time
	for {
		err := runAttempt(name, o, ctx, fn)
		if err == nil {
			return nil
		}
		if o.parent.Err() != nil {
			return err
		}
		attempt := 1
		if o.restarts {
			recent = recordFailure(recent, o.clock.Now(), o.window)
			attempt = len(recent)
		}
		disabled := o.restarts && len(recent) > o.maxRestarts
		report(o, name, err, attempt, disabled)
		if !o.restarts {
			return err
		}
		if disabled {
			return fmt.Errorf("%w: %s: %w", ErrDisabled, name, err)
		}
		if !waitBackoff(ctx, o.clock, backoffFor(len(recent))) {
			return err
		}
	}
}

// report sends one failure to the reporter and hands the reference back to a PanicError.
func report(o options, name string, err error, attempt int, disabled bool) {
	var panicked *PanicError
	isPanic := errors.As(err, &panicked)
	failure := Failure{Name: name, Panic: isPanic, Err: err, Attempt: attempt, Disabled: disabled}
	if isPanic {
		failure.Stack = panicked.Stack
	}
	ref := o.reporter.Report(failure)
	if isPanic {
		panicked.Ref = ref
	}
}

// runAttempt calls fn once, recovering a panic, under the per-attempt timeout when one is set.
func runAttempt(name string, o options, parent context.Context, fn func(context.Context) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if o.timeout <= 0 {
		return guard(name, ctx, fn)
	}
	ticker := o.clock.NewTicker(o.timeout)
	defer ticker.Stop()
	fired := make(chan struct{})
	watcherDone := make(chan struct{})
	go expireAttempt(ticker, ctx, cancel, fired, watcherDone)
	err := guard(name, ctx, fn)
	cancel()
	<-watcherDone
	select {
	case <-fired:
		return timeoutError(name, o.timeout)
	default:
		return err
	}
}

// expireAttempt cancels the attempt when the ticker fires. It exits as soon as the attempt ends, so the
// caller can wait for it and no goroutine outlives the call.
func expireAttempt(ticker platform.Ticker, ctx context.Context, cancel context.CancelFunc, fired chan<- struct{}, done chan<- struct{}) {
	defer close(done)
	select {
	case <-ticker.C():
		close(fired)
		cancel()
	case <-ctx.Done():
	}
}

// guard runs fn and turns a panic into a *PanicError that carries the stack.
func guard(name string, ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = &PanicError{Name: name, Value: value, Stack: string(debug.Stack())}
		}
	}()
	return fn(ctx)
}

func timeoutError(name string, d time.Duration) error {
	return fmt.Errorf("safego: %s timed out after %s: %w", name, d, context.DeadlineExceeded)
}

// recordFailure keeps the failures inside the window and appends the new one.
func recordFailure(recent []time.Time, now time.Time, window time.Duration) []time.Time {
	kept := recent[:0]
	for _, at := range recent {
		if now.Sub(at) <= window {
			kept = append(kept, at)
		}
	}
	return append(kept, now)
}

// backoffFor doubles the delay for each failure still inside the window, up to backoffMax.
func backoffFor(failures int) time.Duration {
	delay := backoffBase
	for i := 1; i < failures && delay < backoffMax; i++ {
		delay *= 2
	}
	return min(delay, backoffMax)
}

// waitBackoff waits for the delay on the clock. It reports false when the context ends first.
func waitBackoff(ctx context.Context, clock platform.Clock, delay time.Duration) bool {
	select {
	case <-clock.After(delay):
		return true
	case <-ctx.Done():
		return false
	}
}
