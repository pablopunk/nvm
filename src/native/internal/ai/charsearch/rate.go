package charsearch

import (
	"errors"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// DecisionsPerMinute is how many decision calls the picker may make in any rolling minute: 1.x's per-extension AI
// limit, which the picker's calls share.
const DecisionsPerMinute = 30

// ErrRateLimited is the error of a search that the rolling-minute limit refused. The picker shows nothing for it.
var ErrRateLimited = errors.New("AI rate limit exceeded")

// callWindow counts the decision calls of the last minute.
type callWindow struct {
	mu    sync.Mutex
	clock platform.Clock
	calls []time.Time
}

func newCallWindow(clock platform.Clock) *callWindow {
	return &callWindow{clock: clock}
}

// allow records a call and reports whether it fits the limit. A refused call is not recorded.
func (w *callWindow) allow() bool {
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
	if len(recent) >= DecisionsPerMinute {
		return false
	}
	w.calls = append(recent, now)
	return true
}
