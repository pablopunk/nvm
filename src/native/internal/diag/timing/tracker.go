// Package timing attaches host-owned correlation IDs and records operation completion.
package timing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
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
}

type Completion struct {
	once    sync.Once
	tracker *Tracker
	ids     correlation
	started time.Time
}

func New(clock platform.Clock, sink Sink) *Tracker {
	return &Tracker{clock: clock, sink: sink, boot: newID()}
}

// Start creates a correlated child operation. Its journey inherits from the parent context.
func (t *Tracker) Start(parent context.Context) (context.Context, *Completion) {
	if parent == nil {
		parent = context.Background()
	}
	prior, _ := parent.Value(contextKey{}).(correlation)
	journey := prior.journey
	if journey == "" {
		journey = newID()
	}
	ids := correlation{boot: t.boot, journey: journey, action: newID()}
	ctx := context.WithValue(parent, contextKey{}, ids)
	return ctx, &Completion{tracker: t, ids: ids, started: t.clock.Now()}
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
		c.tracker.sink.CommandCompleted(analytics.CommandCompletion{
			Outcome: outcome, Duration: c.tracker.clock.Now().Sub(c.started),
			BootID: c.ids.boot, JourneyID: c.ids.journey, ActionID: c.ids.action,
		})
	})
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
