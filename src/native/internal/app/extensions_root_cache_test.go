package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

func TestRootRowCacheAsksOnlyPendingExtensionsOnceFilled(t *testing.T) {
	var c rootRowCache
	c.markPending("a")
	if scope := c.takeScope(); scope != nil {
		t.Fatalf("an unfilled cache asked the scope %v; it must ask every extension", scope)
	}
	fillRootCache(&c, "a", "b")
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
	fillRootCache(&c, "a")
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

// fillRootCache marks c filled with a row for each extension id, as a full root pass leaves it.
func fillRootCache(c *rootRowCache, ids ...string) {
	c.filled = true
	c.rows = map[string]manager.SearchResult{}
	for _, id := range ids {
		c.rows[id] = manager.SearchResult{ExtensionID: id}
	}
}

// TestRootRowCacheDoesNotStoreACancelledAnswer checks that the answer of a pass its context cancelled leaves the rows the
// cache holds, where a real answer replaces them.
func TestRootRowCacheDoesNotStoreACancelledAnswer(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	c.remember(cancelled, 0, manager.SearchResult{ExtensionID: "a", Failure: &api.Failure{Code: api.CodeCancelled}})
	if got := c.rows["a"]; got.Failure != nil {
		t.Fatalf("a cancelled answer replaced the cached rows: %+v", got)
	}
	c.remember(context.Background(), 0, manager.SearchResult{ExtensionID: "a", Title: "fresh"})
	if got := c.rows["a"]; got.Title != "fresh" {
		t.Fatalf("a real answer was not stored: %+v", got)
	}
}

// TestRootRowCacheSettleDropsOnlyTheRowsOfSilentExtensions checks that a completed full pass drops the rows of the
// extensions that did not answer, and a scoped pass drops only the silent extensions of its scope.
func TestRootRowCacheSettleDropsOnlyTheRowsOfSilentExtensions(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a", "b", "c")
	c.settle(nil, map[string]bool{"a": true})
	if len(c.rows) != 1 || c.rows["a"].ExtensionID != "a" || !c.filled {
		t.Fatalf("full pass left rows %v filled=%v, want only a and filled", c.rows, c.filled)
	}

	fillRootCache(&c, "a", "b", "c")
	c.settle(map[string]bool{"b": true}, map[string]bool{"a": true})
	if _, ok := c.rows["b"]; ok || len(c.rows) != 2 {
		t.Fatalf("scoped pass left rows %v, want a and c", c.rows)
	}
}

// TestRootRowCacheCachedOutsideIsTheRowsNotInScope checks that a scoped refresh emits the cached rows of the other
// extensions, in extension id order.
func TestRootRowCacheCachedOutsideIsTheRowsNotInScope(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "c", "a", "b")
	got := c.cachedOutside(map[string]bool{"b": true})
	if len(got) != 2 || got[0].ExtensionID != "a" || got[1].ExtensionID != "c" {
		t.Fatalf("cachedOutside(b) = %+v, want a then c", got)
	}
}
