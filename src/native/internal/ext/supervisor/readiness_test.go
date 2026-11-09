package supervisor

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCallsWaitForStartingChildAndHonorCancellation(t *testing.T) {
	r := New(Config{}, api.Identity{ID: "acme.test"}, "", nil)
	r.mu.Lock()
	r.state = StateStarting
	r.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.readyForCall(ctx, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait = %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- r.readyForCall(context.Background(), true) }()
	r.mu.Lock()
	r.state = StateRunning
	r.notifyLocked()
	r.mu.Unlock()
	if err := <-done; err != nil {
		t.Fatalf("ready child: %v", err)
	}
}
