package jobs

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// installLocked stops the job's current triggers and starts the ones its definition declares, when it is runnable.
// Timers are created here, under the lock, so their deadlines are fixed when the job is installed. The caller holds
// s.mu.
func (s *Scheduler) installLocked(r *record) {
	s.stopLocked(r)
	if !r.runnable() {
		return
	}
	stop := make(chan struct{})
	r.stop = stop
	now := s.clock.Now()
	for _, trigger := range r.def.Triggers {
		switch trigger.Kind {
		case TriggerStartup:
			r.nextAt = now.Add(trigger.Delay)
			s.startAfter(r.def.ID, stop, trigger.Delay, ReasonStartup)
		case TriggerInterval:
			s.intervalTrigger(r, trigger, stop, now)
		}
	}
}

// stopLocked cancels the job's triggers and pending events. A run in progress is not touched. The caller holds s.mu.
func (s *Scheduler) stopLocked(r *record) {
	if r.stop != nil {
		close(r.stop)
		r.stop = nil
	}
	for index, pending := range r.events {
		close(pending.stop)
		delete(r.events, index)
	}
	r.nextAt = time.Time{}
}

// intervalTrigger starts the interval loop and, when the last finished run is overdue under the missed policy,
// one catch-up run.
func (s *Scheduler) intervalTrigger(r *record, trigger Trigger, stop <-chan struct{}, now time.Time) {
	id := r.def.ID
	first := firstInterval(trigger)
	r.nextAt = now.Add(first)
	if missedDue(trigger, r.lastFinished, now) {
		s.startAfter(id, stop, trigger.Delay, ReasonMissed)
	}
	firstTick := s.clock.NewTicker(first)
	s.goGuarded("jobs.interval."+id, func() {
		s.intervalLoop(id, trigger, firstTick, stop)
	})
}

// firstInterval is how long an interval job waits before its first run: its delay, or one period.
func firstInterval(trigger Trigger) time.Duration {
	if trigger.Delay > 0 {
		return trigger.Delay
	}
	return trigger.Every
}

// intervalLoop runs the job after the first wait and then every period until stop closes. A run that falls due while
// a previous one still runs is dropped by start, so a period never overlaps itself.
func (s *Scheduler) intervalLoop(id string, trigger Trigger, firstTick platform.Ticker, stop <-chan struct{}) {
	if !s.await(firstTick, stop) {
		return
	}
	s.tick(id, trigger.Every)
	periodic := s.clock.NewTicker(trigger.Every)
	defer periodic.Stop()
	for {
		select {
		case <-stop:
			return
		case <-s.ctx.Done():
			return
		case <-periodic.C():
			s.tick(id, trigger.Every)
		}
	}
}

// tick moves the next-run time one period ahead and asks for an interval run.
func (s *Scheduler) tick(id string, every time.Duration) {
	s.mu.Lock()
	if r, ok := s.jobs[id]; ok {
		r.nextAt = s.clock.Now().Add(every)
	}
	s.mu.Unlock()
	_ = s.start(id, ReasonInterval, nil, false)
}

// startAfter runs the job once for reason after delay, unless stop closes first.
func (s *Scheduler) startAfter(id string, stop <-chan struct{}, delay time.Duration, reason string) {
	timer := s.once(delay)
	s.goGuarded("jobs.once."+id, func() {
		if s.await(timer, stop) {
			_ = s.start(id, reason, nil, false)
		}
	})
}
