package supervisor

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestStopEndsTheLifetimeThatHostWaitsUse checks that stopping an extension cancels the context its host waits, such
// as a confirmation, run under. A later Start gets a fresh lifetime.
func TestStopEndsTheLifetimeThatHostWaitsUse(t *testing.T) {
	r := New(Config{}, api.Identity{ID: "acme.lifetime", Title: "Lifetime"}, "", nil)
	before := r.lifetimeContext()
	select {
	case <-before.Done():
		t.Fatal("the lifetime ended before the runtime stopped")
	default:
	}
	r.Stop()
	select {
	case <-before.Done():
	default:
		t.Fatal("stopping the runtime did not end its lifetime")
	}
	if after := r.lifetimeContext(); after.Err() != nil {
		t.Fatalf("a stopped runtime handed out an ended lifetime: %v", after.Err())
	}
}
