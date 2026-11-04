//go:build !automation

package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// A default build ignores NVM_RIG_BACKEND: the account still starts on production, packaged, so a loopback origin
// is refused. No request leaves the test.
func TestDefaultBuildIgnoresTheRigBackendOverride(t *testing.T) {
	t.Setenv("NVM_RIG_BACKEND", "http://127.0.0.1:9")
	set := fake.NewSet()
	deps := (&App{}).accountDeps(set.Ports(), nil)
	if deps.Origin != nil || !deps.Packaged || deps.Browser == nil {
		t.Fatalf("default build wiring: origin %v packaged %v browser set %v", deps.Origin, deps.Packaged, deps.Browser != nil)
	}
	session, err := auth.New(deps)
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	if snapshot := session.Snapshot(); snapshot.Environment != backend.EnvironmentProduction || snapshot.BaseURL != "https://api.nvm.fyi" {
		t.Fatalf("default build starts on %s %q, want production", snapshot.Environment, snapshot.BaseURL)
	}
	if err := session.SwitchEnvironment(backend.EnvironmentDevelopment, "http://127.0.0.1:9"); err == nil {
		t.Fatal("default build accepted a loopback backend")
	}
}
