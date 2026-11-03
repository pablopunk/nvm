package safego

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Option configures one call of Go, Run or NewGroup.
type Option func(*options)

// Defaults are the process-wide fallbacks for options a call does not pass. The composition root
// installs them once with SetDefaults; tests pass explicit options instead.
type Defaults struct {
	Clock    platform.Clock
	Reporter Reporter
}

type options struct {
	parent      context.Context
	clock       platform.Clock
	reporter    Reporter
	timeout     time.Duration
	restarts    bool
	maxRestarts int
	window      time.Duration
}

var (
	defaultsMu sync.RWMutex
	defaults   Defaults
)

// SetDefaults installs the clock and reporter used when a call does not pass WithClock or WithReporter.
func SetDefaults(d Defaults) {
	defaultsMu.Lock()
	defer defaultsMu.Unlock()
	defaults = d
}

func currentDefaults() Defaults {
	defaultsMu.RLock()
	defer defaultsMu.RUnlock()
	return defaults
}

// resolve applies the defaults and then the call's options. Timeouts and restart backoff need a clock,
// so a call that asks for either without one is a programming error and panics at the call site.
func resolve(opts []Option) options {
	d := currentDefaults()
	o := options{parent: context.Background(), clock: d.Clock, reporter: d.Reporter}
	for _, apply := range opts {
		apply(&o)
	}
	if o.reporter == nil {
		o.reporter = stderrReporter{}
	}
	if o.clock == nil && (o.timeout > 0 || o.restarts) {
		panic("safego: WithTimeout and WithBackoff need a clock: pass WithClock or call SetDefaults")
	}
	return o
}

// WithContext sets the parent context. Cancelling it stops the call and prevents restarts.
func WithContext(parent context.Context) Option {
	return func(o *options) {
		if parent != nil {
			o.parent = parent
		}
	}
}

// WithClock sets the clock that timeouts and restart backoff use.
func WithClock(clock platform.Clock) Option {
	return func(o *options) { o.clock = clock }
}

// WithReporter sets where failures are reported.
func WithReporter(reporter Reporter) Option {
	return func(o *options) { o.reporter = reporter }
}

// WithTimeout cancels the context given to each attempt after d. The function must return once its
// context is done: the call waits for it, so a function that ignores cancellation holds the caller.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithBackoff restarts the function after a panic, a returned error or a timeout. Restarts wait an
// exponentially growing delay. When more than maxRestarts failures happen within window, the call is
// disabled: it reports Disabled and returns an error wrapping ErrDisabled.
func WithBackoff(maxRestarts int, window time.Duration) Option {
	return func(o *options) {
		o.restarts = true
		o.maxRestarts = maxRestarts
		o.window = window
	}
}
