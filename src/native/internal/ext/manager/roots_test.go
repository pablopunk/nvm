package manager

import (
	"context"
	"testing"
)

// TestRootsAreTheRootItemsOfEnabledExtensions: the empty-query rows come from RootItems, and an extension that is
// off contributes none.
func TestRootsAreTheRootItemsOfEnabledExtensions(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	result := resultFor(h.m.Roots(context.Background()), "acme.good")
	if result == nil || result.Failure != nil || len(result.Items) != 1 || result.Items[0].Title != "Good root" {
		t.Fatalf("Roots = %+v; want the one root item of acme.good", result)
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if result := resultFor(h.m.Roots(context.Background()), "acme.good"); result != nil {
		t.Fatalf("a disabled extension returned roots: %+v", result)
	}
}
