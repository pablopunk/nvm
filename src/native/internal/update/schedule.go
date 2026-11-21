package update

import (
	"context"
	"time"
)

// StartupDelay is how long after launch the first check runs, so the palette opens before any network call.
const StartupDelay = 15 * time.Second

// CheckInterval is the time between checks while the app runs.
const CheckInterval = 4 * time.Hour

// Run checks once after StartupDelay and then every CheckInterval, until ctx ends. It returns at once in a build
// that cannot update. Each check runs on this goroutine, so a slow feed never delays the palette.
func (s *Service) Run(ctx context.Context) {
	if !s.engine.Enabled() {
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-s.clock.After(StartupDelay):
	}
	s.Check(ctx)
	ticker := s.clock.NewTicker(CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			s.Check(ctx)
		}
	}
}
