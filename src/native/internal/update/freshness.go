package update

import (
	"context"
	"time"
)

// CheckedAt is when the last check on the current channel finished, whatever it found. It is the zero time before
// the first check and after a channel switch.
func (s *Service) CheckedAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkedAt
}

// CheckIfStale checks when no check on the current channel finished within maxAge, so a page that opens on an old
// answer refreshes it. It reports whether it ran a check; a busy updater or a build that cannot update runs none.
func (s *Service) CheckIfStale(ctx context.Context, maxAge time.Duration) bool {
	if !s.engine.Enabled() || !s.answerOlderThan(maxAge) {
		return false
	}
	_, owned := s.CheckOwned(ctx)
	return owned
}

func (s *Service) answerOlderThan(maxAge time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkedAt.IsZero() || s.now().Sub(s.checkedAt) > maxAge
}

// now is the service clock's time, or the zero time for a service built without a clock.
func (s *Service) now() time.Time {
	if s.clock == nil {
		return time.Time{}
	}
	return s.clock.Now()
}
