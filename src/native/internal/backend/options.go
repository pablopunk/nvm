package backend

import (
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Option configures a Client. Options that a caller does not pass keep the defaults noted on each.
type Option func(*settings)

// settings is the mutable configuration New builds before it creates the client.
type settings struct {
	clock       platform.Clock
	packaged    bool
	identity    identity
	retry       retryPolicy
	jitter      func(time.Duration) time.Duration
	deviceLabel string
}

// defaultDeviceLabel names this client in the backend's device list when the caller does not.
const defaultDeviceLabel = "Nevermind native"

// WithClock sets the clock that paces request deadlines, retry backoff and device polling. It is required:
// New fails without one, so tests advance a fake clock instead of sleeping.
func WithClock(clock platform.Clock) Option {
	return func(s *settings) { s.clock = clock }
}

// WithPackaged marks a packaged build, which refuses development origins and loopback or private-network
// origins. Default false.
func WithPackaged(packaged bool) Option {
	return func(s *settings) { s.packaged = packaged }
}

// WithClientVersion sets the desktop version sent in X-Nevermind-Client-Version and used to judge
// compatibility. Default is a development version that sorts below every release.
func WithClientVersion(version string) Option {
	return func(s *settings) {
		if version != "" {
			s.identity.version = version
		}
	}
}

// WithDeviceLabel sets the label shown for this client in the backend's device sign-in page.
func WithDeviceLabel(label string) Option {
	return func(s *settings) {
		if label != "" {
			s.deviceLabel = label
		}
	}
}

// WithRetryPolicy sets how many attempts an idempotent GET makes and the backoff bounds. attempts counts the
// first try, so 1 disables retries. Default 3 attempts, 250 ms base, 4 s cap.
func WithRetryPolicy(attempts int, base, maxDelay time.Duration) Option {
	return func(s *settings) {
		s.retry = retryPolicy{attempts: attempts, base: base, max: maxDelay}
	}
}

// WithJitter replaces the random jitter applied to each backoff delay. Tests pass the identity function.
// The default returns a value between half of the delay and the whole delay.
func WithJitter(jitter func(time.Duration) time.Duration) Option {
	return func(s *settings) {
		if jitter != nil {
			s.jitter = jitter
		}
	}
}

// validateSettings reports the first setting that cannot work.
func validateSettings(s settings) error {
	if s.clock == nil {
		return fmt.Errorf("backend: New needs a clock; pass WithClock")
	}
	if s.retry.attempts < 1 {
		return fmt.Errorf("backend: retry attempts must be at least 1")
	}
	return nil
}
