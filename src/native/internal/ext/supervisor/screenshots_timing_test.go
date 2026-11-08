package supervisor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// screenshotTiming records aggregate host time without retaining service arguments or replies.
type screenshotTiming struct {
	ServiceHandler
	mu      sync.Mutex
	calls   int
	elapsed time.Duration
}

func (s *screenshotTiming) Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	start := time.Now()
	reply := s.ServiceHandler.Serve(ctx, call)
	s.mu.Lock()
	s.calls++
	s.elapsed += time.Since(start)
	s.mu.Unlock()
	return reply
}

func (s *screenshotTiming) report(t *testing.T, r *Runtime, start time.Time, phase string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	t.Logf("screenshots timing phase=%s wall=%s completed_host_calls=%d host_wall=%s slot_occupied=%t state=%s", phase, time.Since(start), s.calls, s.elapsed, len(r.slots) != 0, r.Status().State)
}
