package fake

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// epoch is where every fake clock starts, so timestamps in golden files are stable.
var epoch = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// Clock is a platform.Clock that moves only when Advance is called. Timers and tickers fire in deadline
// order during Advance, each seeing Now equal to its own deadline, so code observes the same sequence it
// would with the real clock.
//
//	clock := fake.NewClock()
//	ticks := clock.NewTicker(time.Second)
//	go func() { for range ticks.C() { /* work */ } }()
//	clock.AwaitTimers(1)
//	clock.Advance(3 * time.Second) // three ticks are delivered, dropped if the reader is slow
type Clock struct {
	mu     sync.Mutex
	cond   *sync.Cond
	now    time.Time
	seq    int
	timers []*fakeTimer
}

var _ platform.Clock = (*Clock)(nil)

// fakeTimer is one pending After, Sleep or ticker.
type fakeTimer struct {
	seq      int
	deadline time.Time
	period   time.Duration
	ch       chan time.Time
	active   bool
}

// NewClock returns a clock at the fixture epoch, 2026-01-01 09:00 UTC.
func NewClock() *Clock {
	c := &Clock{now: epoch}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// Now returns the fake time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// After returns a channel that receives the deadline once Advance passes it. A non-positive d fires at
// once.
func (c *Clock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addTimerLocked(d, 0).ch
}

// Sleep blocks the caller until Advance passes d. Advance must run on another goroutine.
func (c *Clock) Sleep(d time.Duration) {
	<-c.After(d)
}

// NewTicker returns a ticker with period d. It panics on a non-positive period, as time.NewTicker does.
func (c *Clock) NewTicker(d time.Duration) platform.Ticker {
	if d <= 0 {
		panic("fake: non-positive ticker interval")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return &fakeTicker{clock: c, timer: c.addTimerLocked(d, d)}
}

// Advance moves the clock forward by d, firing every timer whose deadline falls within the window.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	target := c.now.Add(d)
	for {
		next := c.nextDueLocked(target)
		if next == nil {
			break
		}
		c.now = next.deadline
		fireTimer(next)
		if next.period > 0 {
			next.deadline = next.deadline.Add(next.period)
		} else {
			next.active = false
		}
	}
	c.now = target
}

// AwaitTimers blocks until at least n timers or tickers are pending. Tests use it instead of sleeping to
// wait for a goroutine to start waiting on the clock.
func (c *Clock) AwaitTimers(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.activeLocked() < n {
		c.cond.Wait()
	}
}

// AwaitTimerDueIn blocks until an active timer is due exactly d from now. Unlike AwaitTimers it cannot be
// satisfied by an unrelated or abandoned timer (an After that lost a select stays pending forever), so a test
// advancing by d is sure the goroutine it means is the one waiting.
func (c *Clock) AwaitTimerDueIn(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for !c.hasTimerDueLocked(c.now.Add(d)) {
		c.cond.Wait()
	}
}

// hasTimerDueLocked reports whether an active timer's deadline is exactly at. The caller holds c.mu.
func (c *Clock) hasTimerDueLocked(at time.Time) bool {
	for _, timer := range c.timers {
		if timer.active && timer.deadline.Equal(at) {
			return true
		}
	}
	return false
}

// PendingTimers reports how many timers and tickers are active.
func (c *Clock) PendingTimers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activeLocked()
}

// addTimerLocked registers a timer; a non-positive delay fires immediately. The caller holds c.mu.
func (c *Clock) addTimerLocked(d, period time.Duration) *fakeTimer {
	c.seq++
	timer := &fakeTimer{seq: c.seq, deadline: c.now.Add(d), period: period, ch: make(chan time.Time, 1), active: true}
	if d <= 0 {
		fireTimer(timer)
		timer.active = period > 0
		if period > 0 {
			timer.deadline = c.now.Add(period)
		}
	}
	c.timers = append(c.timers, timer)
	c.cond.Broadcast()
	return timer
}

// nextDueLocked returns the active timer with the earliest deadline at or before target, breaking ties by
// creation order. The caller holds c.mu.
func (c *Clock) nextDueLocked(target time.Time) *fakeTimer {
	var next *fakeTimer
	for _, timer := range c.timers {
		if !timer.active || timer.deadline.After(target) {
			continue
		}
		if next == nil || timer.deadline.Before(next.deadline) || (timer.deadline.Equal(next.deadline) && timer.seq < next.seq) {
			next = timer
		}
	}
	return next
}

// activeLocked counts active timers and compacts the list. The caller holds c.mu.
func (c *Clock) activeLocked() int {
	kept := c.timers[:0]
	for _, timer := range c.timers {
		if timer.active {
			kept = append(kept, timer)
		}
	}
	c.timers = kept
	return len(kept)
}

// fireTimer delivers the deadline without blocking: an unread tick is dropped, as with time.Ticker.
func fireTimer(timer *fakeTimer) {
	select {
	case timer.ch <- timer.deadline:
	default:
	}
}

// fakeTicker implements platform.Ticker over a fakeTimer.
type fakeTicker struct {
	clock *Clock
	timer *fakeTimer
}

// C returns the tick channel.
func (t *fakeTicker) C() <-chan time.Time { return t.timer.ch }

// Stop deactivates the ticker.
func (t *fakeTicker) Stop() {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	t.timer.active = false
}
