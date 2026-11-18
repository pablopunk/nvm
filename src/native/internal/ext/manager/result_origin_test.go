package manager

import (
	"context"
	"testing"
)

// TestResultsCarryTheInstallationThatAnswered checks that search and command answers carry the identity of the
// installation whose runtime produced them, so a host that converts them later cannot bind them to a replacement.
func TestResultsCarryTheInstallationThatAnswered(t *testing.T) {
	h := newHarness(t)
	const id = "acme.good"
	contents := fixtureContents(t, id)
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	first := h.m.SnapshotIdentity(id)
	searched := resultFor(h.search("a"), id)
	if searched == nil || searched.Origin == nil || searched.Origin != first {
		t.Fatalf("search answer origin = %v, want the running installation %v", searched, first)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	var commanded *CommandResult
	for _, result := range h.m.Commands(ctx) {
		if result.ExtensionID == id {
			commanded = &result
		}
	}
	if commanded == nil || commanded.Origin != first {
		t.Fatalf("command answer origin = %+v, want the running installation", commanded)
	}
	if err := h.m.Uninstall(id); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Install(id, contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning(id)
	replacement := h.m.SnapshotIdentity(id)
	if replacement == nil || replacement == searched.Origin {
		t.Fatal("the replacement installation shares the stale answer's identity")
	}
	if fresh := resultFor(h.search("a"), id); fresh == nil || fresh.Origin != replacement {
		t.Fatal("a fresh answer does not carry the replacement's identity")
	}
}
