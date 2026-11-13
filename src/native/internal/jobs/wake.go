package jobs

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// watchWake emits EventWake when a heartbeat arrives much later than the previous one, which means the machine slept
// in between. Overdue interval ticks fire once after the wake (the clock port coalesces them), so a sleep never
// produces a burst of runs. The ticker and the first reading are taken by New, before any goroutine starts.
func (s *Scheduler) watchWake(heartbeat platform.Ticker, last time.Time) {
	defer heartbeat.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case beat := <-heartbeat.C():
			if slept(last, beat, s.sleepGap) {
				s.Emit(EventWake, nil)
			}
			last = beat
		}
	}
}

// slept reports whether the gap between two heartbeats is longer than the sleep threshold. It compares wall clocks:
// the monotonic reading of a time stops while the machine sleeps on macOS, so Sub would see no gap at all.
func slept(previous, current time.Time, gap time.Duration) bool {
	return current.Round(0).Sub(previous.Round(0)) > gap
}
