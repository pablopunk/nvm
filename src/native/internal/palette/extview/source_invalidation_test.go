package extview

import (
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
	if len(loads) != 2 {
		t.Fatalf("loads = %d", len(loads))
	}
	loads[1]([]api.Item{{ID: "fresh"}}, nil)
	loads[0]([]api.Item{{ID: "stale"}}, nil)
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
	cache.Put("test/key", CacheEntry{Items: []api.Item{{ID: "old"}}, At: clock.Now()})
	generation := cache.Generation("test")
	cache.InvalidateExtension("test")
	if cache.PutGeneration("test", "test/key", generation, CacheEntry{}) {
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
	old.revalidateMutation("test")
	// A late service call carries the old invocation identity.
	cache.InvalidateExtension(cache.Scope("test", first))
	loads[0]([]api.Item{{ID: "old"}}, nil)
	reopened := NewSession(deps, "test", view)
	reopened.Open(&removalHost{})
	if len(loads) != 2 || len(reopened.items) != 1 || reopened.items[0].ID != "replacement" {
		t.Fatalf("loads=%d replacement items=%v", len(loads), reopened.items)
	}
}

func TestScopeEvictionRejectsOldWritesAndBoundsState(t *testing.T) {
	cache := NewMemoryCache()
	oldIdentity := new(int)
	old := cache.Scope("test", oldIdentity)
	oldGeneration := cache.Generation(old)
	cache.PutGeneration(old, old+"/key", oldGeneration, CacheEntry{})
	for i := 0; i < maxSourceScopes*3; i++ {
		identity := new(int)
		scope := cache.Scope("test", identity)
		cache.PutGeneration(scope, scope+"/key", cache.Generation(scope), CacheEntry{})
	}
	if len(cache.scopes) > maxSourceScopes || len(cache.generations) > maxSourceScopes || len(cache.entries) > maxSourceEntries {
		t.Fatalf("unbounded scopes=%d generations=%d entries=%d", len(cache.scopes), len(cache.generations), len(cache.entries))
	}
	replacement := cache.Scope("test", oldIdentity)
	if replacement == old {
		t.Fatal("evicted identity reused its scope")
	}
	if cache.PutGeneration(old, old+"/key", oldGeneration, CacheEntry{}) {
		t.Fatal("evicted write accepted")
	}
	if _, ok := cache.Get(replacement + "/key"); ok {
		t.Fatal("replacement inherited old snapshot")
	}
}
