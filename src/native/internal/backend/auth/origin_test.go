package auth

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// A session given an origin starts on it, and the stored profile keeps the backend it named before.
func TestSessionStartsOnTheGivenOriginAndKeepsTheStoredProfile(t *testing.T) {
	origin, err := backend.NewEnvironment(backend.EnvironmentDevelopment, "http://127.0.0.1:9", false)
	if err != nil {
		t.Fatalf("NewEnvironment: %v", err)
	}
	r := newRig(t, rigOptions{origin: &origin})

	snapshot := r.session.Snapshot()
	if snapshot.Environment != backend.EnvironmentDevelopment || snapshot.BaseURL != "http://127.0.0.1:9" {
		t.Fatalf("session starts on %s %q, want the given development origin", snapshot.Environment, snapshot.BaseURL)
	}
	stored, err := r.profile.Load()
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if stored.Environment != backend.EnvironmentProduction || stored.BaseURL != "" {
		t.Fatalf("stored profile changed to %s %q", stored.Environment, stored.BaseURL)
	}
}
