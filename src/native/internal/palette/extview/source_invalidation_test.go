package extview

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestMutationInvalidatesScopedSnapshotsAndFencesLoads(t *testing.T) {
	clock := fake.NewClock()
	cache := NewMemoryCache()
	cache.Put("other/key", CacheEntry{Items: []api.Item{{ID: "other"}}, At: clock.Now()})
	var loads []func([]api.Item, *api.Failure)
	var mutation func(api.Result, *api.Failure)
	deps := Deps{Clock: clock, Cache: cache, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }, Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) { mutation = done }}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	s := NewSession(deps, "test", view)
	h := &removalHost{}
	s.Open(h)
	s.submit(h, api.Invocation{}, nil)
	mutation(api.Result{DataChanged: true}, nil)
	if len(loads) != 1 {
		t.Fatalf("loads = %d", len(loads))
	}
	loads[0]([]api.Item{{ID: "stale"}}, nil)
	if len(loads) != 2 {
		t.Fatalf("coalesced loads = %d", len(loads))
	}
	loads[1]([]api.Item{{ID: "fresh"}}, nil)
	entry, ok := cache.Get("test/key")
	if !ok || entry.Items[0].ID != "fresh" || s.items[0].ID != "fresh" {
		t.Fatalf("entry=%v items=%v", entry, s.items)
	}
	if _, ok := cache.Get("other/key"); !ok {
		t.Fatal("unrelated snapshot removed")
	}
	reopened := NewSession(deps, "test", view)
	reopened.Open(h)
	if len(loads) != 2 || reopened.items[0].ID != "fresh" {
		t.Fatalf("next open: loads=%d items=%v", len(loads), reopened.items)
	}
}

func TestInvalidationRejectsLateWritesAndCancellation(t *testing.T) {
	clock := fake.NewClock()
	cache := NewMemoryCache()
	lease := cache.acquire("test", nil)
	cache.Put("test/key", CacheEntry{Items: []api.Item{{ID: "old"}}, At: clock.Now()})
	generation := lease.version()
	cache.invalidate("test", nil)
	if lease.put("key", generation, CacheEntry{}) != sourceStale {
		t.Fatal("accepted stale write")
	}
	var load func([]api.Item, *api.Failure)
	s := NewSession(Deps{Clock: clock, Cache: cache, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { load = done }}, "test", api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}})
	page := s.Open(&removalHost{})
	page.OnLeave()
	load([]api.Item{{ID: "late"}}, nil)
	if _, ok := cache.Get("test/key"); ok {
		t.Fatal("cancelled load populated cache")
	}
}

func TestMutationAfterDismissalInvalidatesNextOpen(t *testing.T) {
	clock := fake.NewClock()
	cache := NewMemoryCache()
	cache.Put("test/key", CacheEntry{Items: []api.Item{{ID: "old"}}, At: clock.Now()})
	var mutation func(api.Result, *api.Failure)
	var load func([]api.Item, *api.Failure)
	deps := Deps{Clock: clock, Cache: cache, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { load = done }, Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) { mutation = done }}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	s := NewSession(deps, "test", view)
	h := &removalHost{}
	page := s.Open(h)
	s.submit(h, api.Invocation{}, nil)
	page.OnLeave()
	mutation(api.Result{DataChanged: true}, &api.Failure{Message: "after mutation"})
	reopened := NewSession(deps, "test", view)
	reopened.Open(h)
	if load == nil {
		t.Fatal("next open reused invalidated snapshot")
	}
	load([]api.Item{{ID: "new"}}, nil)
	if len(reopened.items) != 1 || reopened.items[0].ID != "new" {
		t.Fatalf("items=%v", reopened.items)
	}
}

func TestReplacementInstallationKeepsItsOwnSnapshot(t *testing.T) {
	clock := fake.NewClock()
	cache := NewMemoryCache()
	first, second := new(int), new(int)
	current := any(first)
	var loads []func([]api.Item, *api.Failure)
	deps := Deps{
		Clock: clock, Cache: cache,
		CommandSnapshotIdentity: func(string) any { return current },
		Load:                    func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) },
	}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	old := NewSession(deps, "test", view)
	old.Open(&removalHost{})
	current = second
	replacement := NewSession(deps, "test", view)
	replacement.Open(&removalHost{})
	loads[1]([]api.Item{{ID: "replacement"}}, nil)
	old.revalidateMutation("test", first)
	// A late service call carries the old invocation identity.
	cache.invalidate("test", first)
	loads[0]([]api.Item{{ID: "old"}}, nil)
	reopened := NewSession(deps, "test", view)
	reopened.Open(&removalHost{})
	if len(loads) != 2 || len(reopened.items) != 1 || reopened.items[0].ID != "replacement" {
		t.Fatalf("loads=%d replacement items=%v", len(loads), reopened.items)
	}
}

func TestExplicitInvalidationRefreshesVisibleMatchingInstallation(t *testing.T) {
	clock := fake.NewClock()
	cache := NewMemoryCache()
	identity, replacement := new(int), new(int)
	current := any(identity)
	var loads []func([]api.Item, *api.Failure)
	deps := Deps{Clock: clock, Cache: cache, CommandSnapshotIdentity: func(string) any { return current }, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	session := NewSession(deps, "test", view)
	page := session.Open(&removalHost{})
	loads[0]([]api.Item{{ID: "before"}}, nil)
	cache.invalidate("test", identity)
	page.OnSourceInvalidated(replacement)
	if len(loads) != 1 {
		t.Fatal("other installation refreshed view")
	}
	page.OnSourceInvalidated(identity)
	if len(loads) != 2 {
		t.Fatalf("loads=%d", len(loads))
	}
	loads[1]([]api.Item{{ID: "after"}}, nil)
	if len(session.items) != 1 || session.items[0].ID != "after" {
		t.Fatalf("items=%v", session.items)
	}
	page.OnLeave()
	page.OnSourceInvalidated(identity)
	if len(loads) != 2 {
		t.Fatal("dismissed view reloaded")
	}
}

func TestSourceLoadContextCancelledOnDismissal(t *testing.T) {
	var loadContext context.Context
	identity := new(int)
	deps := Deps{
		CommandSnapshotIdentity: func(string) any { return identity },
		LoadContext: func(ctx context.Context, id, loader string, got any, done func([]api.Item, *api.Failure)) {
			if id != "test" || loader != "load" || got != identity {
				t.Fatalf("load id=%s loader=%s identity=%v", id, loader, got)
			}
			loadContext = ctx
		},
	}
	session := NewSession(deps, "test", api.View{Kind: api.ViewList, Source: &api.DataSource{Loader: "load"}})
	page := session.Open(&removalHost{})
	if loadContext == nil || loadContext.Err() != nil {
		t.Fatal("loader context unavailable")
	}
	page.OnLeave()
	if loadContext.Err() != context.Canceled {
		t.Fatalf("loader context error=%v", loadContext.Err())
	}
}

func TestCoveredSourceRevalidatesOnReveal(t *testing.T) {
	cache := NewMemoryCache()
	clock := fake.NewClock()
	var loads []func([]api.Item, *api.Failure)
	deps := Deps{Cache: cache, Clock: clock, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	session := NewSession(deps, "test", view)
	page := session.Open(&removalHost{})
	loads[0]([]api.Item{{ID: "old"}}, nil)
	cache.invalidate("test", nil)
	if page.OnReveal == nil {
		t.Fatal("source page has no reveal callback")
	}
	page.OnReveal()
	if len(loads) != 2 || session.items[0].ID != "old" {
		t.Fatalf("loads=%d items=%v", len(loads), session.items)
	}
	loads[1]([]api.Item{{ID: "new"}}, nil)
	if session.items[0].ID != "new" {
		t.Fatalf("items=%v", session.items)
	}
	page.OnLeave()
}

func TestUncacheableSourceStillPresentsItems(t *testing.T) {
	cache := NewMemoryCache()
	cache.maxBytes = 1
	var loads []func([]api.Item, *api.Failure)
	deps := Deps{Cache: cache, Clock: fake.NewClock(), Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	session := NewSession(deps, "test", view)
	page := session.Open(&removalHost{})
	loads[0]([]api.Item{{ID: "oversized"}}, nil)
	if len(loads) != 1 || session.loading || len(session.items) != 1 || session.items[0].ID != "oversized" {
		t.Fatalf("loads=%d loading=%v items=%v", len(loads), session.loading, session.items)
	}
	page.OnLeave()
	reopened := NewSession(deps, "test", view)
	reopened.Open(&removalHost{})
	if len(loads) != 2 {
		t.Fatalf("uncacheable snapshot reused: loads=%d", len(loads))
	}
}

func TestLiveEpochSurvivesEvictionAndMutation(t *testing.T) {
	cache := NewMemoryCache()
	cache.maxEntries = 1
	clock := fake.NewClock()
	var loads []func([]api.Item, *api.Failure)
	deps := Deps{Cache: cache, Clock: clock, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}}
	first := NewSession(deps, "test", view)
	firstPage := first.Open(&removalHost{})
	loads[0]([]api.Item{{ID: "old"}}, nil)
	other := cache.acquire("other", nil)
	other.put("key", other.version(), CacheEntry{Items: []api.Item{{ID: "other"}}})
	other.release()
	second := NewSession(deps, "test", view)
	second.Open(&removalHost{})
	loads[1]([]api.Item{{ID: "second"}}, nil)
	before := first.sourceLease.version()
	first.revalidateMutation("test", nil)
	if first.sourceLease.version() == before || second.sourceLease.version() == before {
		t.Fatal("live sessions did not share invalidation epoch")
	}
	if len(loads) != 3 {
		t.Fatalf("loads=%d", len(loads))
	}
	loads[2]([]api.Item{{ID: "fresh"}}, nil)
	firstPage.OnLeave()
}
