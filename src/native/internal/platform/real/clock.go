package real

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// realClock is the wall clock from package time.
type realClock struct{}

// Now returns the current time.
func (realClock) Now() time.Time { return time.Now() }

// After returns a channel that receives once after d.
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// NewTicker returns a ticker backed by time.NewTicker.
func (realClock) NewTicker(d time.Duration) platform.Ticker {
	return realTicker{ticker: time.NewTicker(d)}
}

// Sleep blocks for d.
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// realTicker adapts *time.Ticker to platform.Ticker.
type realTicker struct {
	ticker *time.Ticker
}

// C returns the tick channel.
func (t realTicker) C() <-chan time.Time { return t.ticker.C }

// Stop stops the ticker.
func (t realTicker) Stop() { t.ticker.Stop() }
