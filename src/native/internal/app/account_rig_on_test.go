//go:build automation

package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// An automation build starts the account on the origin the rig names, unpackaged, without a browser.
func TestAutomationBuildStartsTheAccountOnTheRigBackend(t *testing.T) {
	t.Setenv(rigBackendEnv, "http://127.0.0.1:4555")
	deps := (&App{}).accountDeps(fake.NewSet().Ports(), nil)
	if deps.Packaged || deps.Browser != nil {
		t.Fatalf("rig wiring: packaged %v browser set %v", deps.Packaged, deps.Browser != nil)
	}
	session, err := auth.New(deps)
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	if snapshot := session.Snapshot(); snapshot.Environment != backend.EnvironmentDevelopment || snapshot.BaseURL != "http://127.0.0.1:4555" {
		t.Fatalf("rig build starts on %s %q", snapshot.Environment, snapshot.BaseURL)
	}
}

// Without a named backend the automation build talks to the discard port, never to production.
func TestAutomationBuildWithoutABackendStaysOffProduction(t *testing.T) {
	t.Setenv(rigBackendEnv, "")
	session, err := auth.New((&App{}).accountDeps(fake.NewSet().Ports(), nil))
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	if got := session.Snapshot().BaseURL; got != rigUnreachableOrigin {
		t.Fatalf("rig build without a backend uses %q, want %q", got, rigUnreachableOrigin)
	}
}
