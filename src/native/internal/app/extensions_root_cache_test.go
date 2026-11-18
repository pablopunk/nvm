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
	if scope, _ := c.takeScope(); scope != nil {
		t.Fatalf("an unfilled cache asked the scope %v; it must ask every extension", scope)
	}
	fillRootCache(&c, "a", "b")
	c.markPending("a")
	scope, _ := c.takeScope()
	if !scope["a"] || scope["b"] || len(scope) != 1 {
		t.Fatalf("scope = %v, want only a", scope)
	}
	if again, _ := c.takeScope(); again != nil {
		t.Fatalf("the pending scope was not consumed: %v", again)
	}
}

func TestRootRowCacheResetAsksEveryExtension(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a")
	c.markPending("a")
	c.reset()
	if scope, _ := c.takeScope(); scope != nil {
		t.Fatalf("a reset cache asked the scope %v; it must ask every extension", scope)
	}
}

// TestInvalidateRootRefreshesTheCallersRootRowsOnly runs the acme.fallback fixture, whose root row counts its RootItems
// calls, next to acme.good. A refresh asked by acme.fallback runs its root again and shows the new count.
func TestInvalidateRootRefreshesTheCallersRootRowsOnly(t *testing.T) {
	m := newManagerOver(t, nil, "acme.fallback", "acme.good")
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
	c.settle(context.Background(), c.epoch, nil, map[string]bool{"a": true})
	if len(c.rows) != 1 || c.rows["a"].ExtensionID != "a" || !c.filled {
		t.Fatalf("full pass left rows %v filled=%v, want only a and filled", c.rows, c.filled)
	}

	fillRootCache(&c, "a", "b", "c")
	c.settle(context.Background(), c.epoch, map[string]bool{"b": true}, map[string]bool{"a": true})
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

func TestRootRowCacheResetRejectsObsoleteSettlement(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a", "b")
	scope, epoch := c.takeScope()
	c.reset()
	removed := c.settle(context.Background(), epoch, scope, map[string]bool{"a": true})
	if len(removed) != 0 || len(c.rows) != 2 || c.filled {
		t.Fatalf("obsolete settlement changed reset cache: removed=%v rows=%v filled=%v", removed, c.rows, c.filled)
	}
	if c.remember(context.Background(), epoch, manager.SearchResult{ExtensionID: "a", Title: "obsolete"}) {
		t.Fatal("obsolete answer was accepted after reset")
	}
	rows := c.cachedOutside(nil)
	if len(rows) != 2 || rows[0].ExtensionID != "a" || rows[1].ExtensionID != "b" {
		t.Fatalf("full refresh lost stale snapshot: %+v", rows)
	}
}

func TestRootRowCacheCancelledPassCannotSettle(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a", "b")
	scope, epoch := c.takeScope()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if removed := c.settle(ctx, epoch, scope, nil); len(removed) != 0 || len(c.rows) != 2 {
		t.Fatalf("cancelled pass removed cached rows: removed=%v rows=%v", removed, c.rows)
	}
}

func TestCatalogRefreshPublishesOneCompleteSnapshot(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a", "b")
	c.rows["a"] = manager.SearchResult{ExtensionID: "a", Title: "old a"}
	c.rows["b"] = manager.SearchResult{ExtensionID: "b", Title: "old b"}
	scope, epoch := c.takeScope()
	staged := map[string]manager.SearchResult{"a": {ExtensionID: "a", Title: "new a"}}
	before := c.cachedOutside(nil)
	if before[0].Title != "old a" || before[1].Title != "old b" {
		t.Fatalf("partial refresh leaked: %+v", before)
	}
	staged["b"] = manager.SearchResult{ExtensionID: "b", Title: "new b"}
	c.commitPass(context.Background(), epoch, scope, staged)
	after := c.cachedOutside(nil)
	if after[0].Title != "new a" || after[1].Title != "new b" {
		t.Fatalf("complete refresh = %+v", after)
	}
}

func TestCatalogRefreshRejectsCancellationAndReset(t *testing.T) {
	for _, reset := range []bool{false, true} {
		var c rootRowCache
		fillRootCache(&c, "a")
		_, epoch := c.takeScope()
		ctx, cancel := context.WithCancel(context.Background())
		if reset {
			c.reset()
		} else {
			cancel()
		}
		changed := c.commitPass(ctx, epoch, nil, map[string]manager.SearchResult{"a": {ExtensionID: "a", Title: "obsolete"}})
		cancel()
		if len(changed) != 0 || c.cachedOutside(nil)[0].Title == "obsolete" {
			t.Fatal("obsolete catalog was published")
		}
	}
}

func TestNewCatalogPassSupersedesOlderPass(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a")
	_, old := c.takeScope()
	_, current := c.takeScope()
	c.commitPass(context.Background(), current, nil, map[string]manager.SearchResult{"a": {ExtensionID: "a", Title: "new"}})
	c.commitPass(context.Background(), old, nil, map[string]manager.SearchResult{"a": {ExtensionID: "a", Title: "old"}})
	if got := c.cachedOutside(nil)[0].Title; got != "new" {
		t.Fatalf("superseded pass replaced catalog: %q", got)
	}
}

func TestCatalogReadersSeeOldOrCompleteRefresh(t *testing.T) {
	var c rootRowCache
	fillRootCache(&c, "a", "b")
	c.rows["a"] = manager.SearchResult{ExtensionID: "a", Title: "old"}
	c.rows["b"] = manager.SearchResult{ExtensionID: "b", Title: "old"}
	_, epoch := c.takeScope()
	stageA := make(chan struct{})
	stageB := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		answers := map[string]manager.SearchResult{"a": {ExtensionID: "a", Title: "new"}}
		close(stageA)
		<-stageB
		answers["b"] = manager.SearchResult{ExtensionID: "b", Title: "new"}
		c.commitPass(context.Background(), epoch, nil, answers)
	}()
	<-stageA
	rows := c.cachedOutside(nil)
	if rows[0].Title != "old" || rows[1].Title != "old" {
		t.Fatalf("partial catalog: %+v", rows)
	}
	close(stageB)
	<-done
	rows = c.cachedOutside(nil)
	if rows[0].Title != "new" || rows[1].Title != "new" {
		t.Fatalf("incomplete catalog: %+v", rows)
	}
}
