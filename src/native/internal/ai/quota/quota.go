// Package quota counts model calls per minute. The emoji picker's decisions and each extension's AI calls draw from
// the same count for the same owner, as 1.x's per-extension AI limit does.
package quota

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// CallsPerMinute is 1.x's per-extension AI limit: the model calls one owner may make in any rolling minute.
const CallsPerMinute = 30

// Window counts the calls of one owner in the last minute. A refused call is not recorded.
type Window struct {
	mu        sync.Mutex
	clock     platform.Clock
	perMinute int
	calls     []time.Time
}

// NewWindow returns a window that allows perMinute calls in any rolling minute.
func NewWindow(clock platform.Clock, perMinute int) *Window {
	return &Window{clock: clock, perMinute: perMinute}
}

// Allow records a call and reports whether it fits the limit.
func (w *Window) Allow() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	recent := make([]time.Time, 0, len(w.calls)+1)
	for _, at := range w.calls {
		if now.Sub(at) < time.Minute {
			recent = append(recent, at)
		}
	}
	w.calls = recent
	if len(recent) >= w.perMinute {
		return false
	}
	w.calls = append(recent, now)
	return true
}

// Registry hands out one window per owner ID, so every caller that counts against an owner shares its count.
type Registry struct {
	mu        sync.Mutex
	clock     platform.Clock
	perMinute int
	windows   map[string]*Window
}

// NewRegistry returns a registry whose windows each allow perMinute calls.
func NewRegistry(clock platform.Clock, perMinute int) *Registry {
	return &Registry{clock: clock, perMinute: perMinute, windows: map[string]*Window{}}
}

// For returns the window of the owner, created on first use.
func (r *Registry) For(owner string) *Window {
	r.mu.Lock()
	defer r.mu.Unlock()
	if w, ok := r.windows[owner]; ok {
		return w
	}
	w := NewWindow(r.clock, r.perMinute)
	r.windows[owner] = w
	return w
}
