package supervisor

import (
	"context"
	"sync"
)

// dispatchGate defines when a request becomes in flight. Cancellation before admission prevents dispatch; an
// ordinary request admitted first can finish even when its caller later stops waiting.
type dispatchGate struct {
	mu        sync.Mutex
	cancelled bool
}

func (g *dispatchGate) cancel() {
	g.mu.Lock()
	g.cancelled = true
	g.mu.Unlock()
}

func (g *dispatchGate) admit(ctx, lifetime context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancelled {
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if lifetime.Err() != nil {
		return unavailable("stopped")
	}
	return nil
}
