package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// TestDisableRowStopsTheRunningChild runs the row's Disable on a real manager and checks that the child is gone
// and the extension is off: the action reaches the manager, not just a flag in the view.
func TestDisableRowStopsTheRunningChild(t *testing.T) {
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	h := &viewHost{}
	actionTitled(t, extensionListRow(&App{}, m, listingFor(t, m, "acme.good")), "Disable").Run(h)

	after := listingFor(t, m, "acme.good")
	if after.Enabled || after.Runtime.State != supervisor.StateIdle {
		t.Fatalf("after Disable: enabled %v state %s, want off and idle", after.Enabled, after.Runtime.State)
	}
}
