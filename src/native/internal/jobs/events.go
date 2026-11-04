package jobs

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// reasonEventPrefix is the prefix of the Reason of a run that an event started, as in "event:clipboard.changed".
const reasonEventPrefix = "event:"

// Emit reports that an event happened. Every runnable job with an event trigger for it is debounced: a burst of
// events runs the job once, after the quiet period, and never later than the trigger's MaxWait after the first.
func (s *Scheduler) Emit(event string, payload Payload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	now := s.clock.Now()
	for _, r := range s.jobs {
		if !r.runnable() {
			continue
		}
		for index, trigger := range r.def.Triggers {
			if trigger.Kind == TriggerEvent && trigger.Event == event {
				s.debounceLocked(r, index, trigger, payload, now)
			}
		}
	}
}

// debounceLocked restarts the quiet period of one event trigger. The caller holds s.mu.
func (s *Scheduler) debounceLocked(r *record, index int, trigger Trigger, payload Payload, now time.Time) {
	pending, ok := r.events[index]
	if !ok {
		pending = &pendingEvent{firstAt: now}
		r.events[index] = pending
	} else {
		close(pending.stop)
	}
	pending.stop = make(chan struct{})
	pending.payload = mergePayloads(pending.payload, payload)
	id, stop := r.def.ID, pending.stop
	timer := s.once(debounceDelay(trigger, pending.firstAt, now))
	reason := reasonEventPrefix + trigger.Event
	s.goGuarded("jobs.event."+id, func() {
		s.fireEvent(id, index, stop, timer, reason)
	})
}

// fireEvent runs the job for a debounced event once its quiet period passes. A newer event for the same trigger
// replaces stop, so an older timer does nothing.
func (s *Scheduler) fireEvent(id string, index int, stop chan struct{}, timer platform.Ticker, reason string) {
	if !s.await(timer, stop) {
		return
	}
	s.mu.Lock()
	r, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	pending, ok := r.events[index]
	if !ok || pending.stop != stop {
		s.mu.Unlock()
		return
	}
	delete(r.events, index)
	payload := pending.payload
	s.mu.Unlock()
	_ = s.start(id, reason, payload, false)
}
