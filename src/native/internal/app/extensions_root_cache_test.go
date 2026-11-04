package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

func TestRootRowCacheAsksOnlyPendingExtensionsOnceFilled(t *testing.T) {
	var c rootRowCache
	c.markPending("a")
	if scope := c.takeScope(); scope != nil {
		t.Fatalf("an unfilled cache asked the scope %v; it must ask every extension", scope)
	}
	c.storeAll([]manager.SearchResult{{ExtensionID: "a"}, {ExtensionID: "b"}})
	c.markPending("a")
	scope := c.takeScope()
	if !scope["a"] || scope["b"] || len(scope) != 1 {
		t.Fatalf("scope = %v, want only a", scope)
	}
	if again := c.takeScope(); again != nil {
		t.Fatalf("the pending scope was not consumed: %v", again)
	}
}

func TestRootRowCacheResetAsksEveryExtension(t *testing.T) {
	var c rootRowCache
	c.storeAll([]manager.SearchResult{{ExtensionID: "a"}})
	c.markPending("a")
	c.reset()
	if scope := c.takeScope(); scope != nil {
		t.Fatalf("a reset cache asked the scope %v; it must ask every extension", scope)
	}
}

// TestInvalidateRootRefreshesTheCallersRootRowsOnly runs the acme.fallback fixture, whose root row counts its RootItems
// calls, next to acme.good. A refresh asked by acme.fallback runs its root again and shows the new count.
func TestInvalidateRootRefreshesTheCallersRootRowsOnly(t *testing.T) {
	m := newFixtureManager(t, "acme.fallback", "acme.good")
	waitForState(t, m, "acme.fallback", supervisor.StateRunning)
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := goldenPalette(t)
	p.exts.mgr.Store(m)

	findRowTitled(t, rootResultsForTable(p, ""), "Root run 1")
	p.rootRows.markPending("acme.fallback")
	rows := rootResultsForTable(p, "")
	findRowTitled(t, rows, "Root run 2")
	findRowTitled(t, rows, "Good root")
}
