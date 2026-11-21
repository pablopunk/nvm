package apps

import (
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Poll calls fn every interval until the returned stop function is called. The palette uses it for the
// periodic refresh while it is visible, so nothing polls while the window is hidden. stop is idempotent and
// returns once no further call of fn can start.
func Poll(clock platform.Clock, interval time.Duration, fn func()) (stop func()) {
	ticker := clock.NewTicker(interval)
	done := make(chan struct{})
	finished := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(finished)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C():
				fn()
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
		<-finished
	}
}
