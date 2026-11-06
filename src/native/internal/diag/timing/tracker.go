// Package timing attaches host-owned correlation IDs and records operation completion.
package timing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/platform"
)

type Sink interface {
	CommandCompleted(analytics.CommandCompletion) bool
}

type Tracker struct {
	clock platform.Clock
	sink  Sink
	boot  string
}

type contextKey struct{}

type correlation struct {
	boot, journey, action string
	background            bool
}

type Completion struct {
	once         sync.Once
	tracker      *Tracker
	ids          correlation
	started      time.Time
	job          bool
	session      analytics.SessionSnapshot
	usage        bool
	usageVariant string
}

func New(clock platform.Clock, sink Sink) *Tracker {
	return &Tracker{clock: clock, sink: sink, boot: newID()}
}

// Start creates a correlated child operation. Its journey inherits from the parent context.
func (t *Tracker) Start(parent context.Context) (context.Context, *Completion) {
	return t.start(parent, false)
}

// StartCommand opts an explicitly marked extension command into feature usage reporting.
func (t *Tracker) StartCommand(parent context.Context, origin manager.CommandOrigin) (context.Context, *Completion) {
	ctx, completion := t.start(parent, false)
	completion.usage = true
	completion.usageVariant = "user-extension"
	if origin.ExtensionID() != "" {
		if variant, ok := analytics.BuiltInCommandVariant("extension-command", origin.ExtensionID(), origin.CommandID()); ok {
			completion.usageVariant = variant
		}
	}
	return ctx, completion
}

// StartNativeCommand opts a native command into fixed-label usage reporting.
func (t *Tracker) StartNativeCommand(parent context.Context, variant string) (context.Context, *Completion) {
	ctx, completion := t.start(parent, false)
	completion.usage = true
	completion.usageVariant = variant
	return ctx, completion
}

func (t *Tracker) start(parent context.Context, freshJourney bool) (context.Context, *Completion) {
	if parent == nil {
		parent = context.Background()
	}
	prior, _ := parent.Value(contextKey{}).(correlation)
	journey := prior.journey
	if freshJourney {
		journey = ""
	}
	if journey == "" {
		journey = newID()
	}
	ids := correlation{boot: t.boot, journey: journey, action: newID(), background: prior.background || freshJourney}
	ctx := context.WithValue(parent, contextKey{}, ids)
	completion := &Completion{tracker: t, ids: ids, started: t.clock.Now()}
	if !ids.background {
		if source, ok := t.sink.(interface {
			TouchSession() (analytics.SessionSnapshot, bool)
		}); ok {
			completion.session, _ = source.TouchSession()
		}
	}
	return ctx, completion
}

// Finish records once, at the moment the caller knows the underlying operation completed.
func (c *Completion) Finish(outcome analytics.OperationOutcome) {
	if c == nil || c.tracker == nil {
		return
	}
	c.once.Do(func() {
		if c.tracker.sink == nil {
			return
		}
		completion := analytics.Completion{
			Outcome: outcome, Duration: c.tracker.clock.Now().Sub(c.started),
			BootID: c.ids.boot, JourneyID: c.ids.journey, ActionID: c.ids.action,
			Session: c.session,
		}
		if c.job {
			if jobSink, ok := c.tracker.sink.(interface {
				JobCompleted(analytics.JobCompletion) bool
			}); ok {
				jobSink.JobCompleted(completion)
			}
		} else {
			c.tracker.sink.CommandCompleted(completion)
			if c.usage && !c.ids.background {
				if usageSink, ok := c.tracker.sink.(interface {
					CommandUsed(analytics.Completion, string) bool
				}); ok {
					usageSink.CommandUsed(completion, c.usageVariant)
				}
			}
		}
	})
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
