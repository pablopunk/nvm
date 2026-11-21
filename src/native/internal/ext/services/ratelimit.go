package services

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Per-extension call rates. A bucket starts full, so a burst up to the limit is allowed, then calls refill at one
// limit per minute.
const (
	fetchesPerMinute   = 60
	logWritesPerMinute = 300
)

// rootRefreshBurst and rootRefreshesPerMinute are 1.x's refresh rule: five refreshes in any two seconds.
const (
	rootRefreshBurst       = 5
	rootRefreshesPerMinute = 150
)

// rateLimiter is a token bucket that reads time only through its clock, so tests advance it without sleeping.
type rateLimiter struct {
	mu         sync.Mutex
	clock      platform.Clock
	capacity   float64
	refillRate float64
	tokens     float64
	refilledAt time.Time
	started    bool
}

func newRateLimiter(clock platform.Clock, perMinute int) *rateLimiter {
	return newBurstRateLimiter(clock, perMinute, perMinute)
}

// newBurstRateLimiter allows burst calls at once, then refills at perMinute.
func newBurstRateLimiter(clock platform.Clock, burst, perMinute int) *rateLimiter {
	return &rateLimiter{clock: clock, capacity: float64(burst), refillRate: float64(perMinute)}
}

// allow takes one token and reports whether the call may run.
func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()
	if !r.started {
		r.started = true
		r.tokens = r.capacity
		r.refilledAt = now
	}
	elapsed := now.Sub(r.refilledAt).Minutes()
	if elapsed > 0 {
		r.tokens = min(r.capacity, r.tokens+elapsed*r.refillRate)
		r.refilledAt = now
	}
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}
