package platform

import "time"

// Clock is the intent "know the time and wait for it to pass". Code that schedules work (debounces,
// tickers, timers, cache expiry, the timer command) takes a Clock so tests advance time instead of
// sleeping. Every OS supports it.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// After returns a channel that receives once after d has passed.
	After(d time.Duration) <-chan time.Time
	// NewTicker returns a ticker that delivers a tick every d until stopped.
	NewTicker(d time.Duration) Ticker
	// Sleep blocks the caller for d.
	Sleep(d time.Duration)
}

// Ticker delivers ticks from a Clock. Ticks that nobody reads are dropped, as with time.Ticker.
type Ticker interface {
	// C returns the channel the ticks arrive on.
	C() <-chan time.Time
	// Stop turns the ticker off. It does not close C.
	Stop()
}
