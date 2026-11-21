package extview

import (
	"reflect"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPartialRefreshDoesNotRenewSourceSnapshot(t *testing.T) {
	patches := []struct {
		name  string
		patch api.ViewPatch
		want  []string
	}{
		{"status", api.ViewPatch{Status: "ready"}, []string{"one/Old", "two/Two"}},
		{"merge", api.ViewPatch{Items: []api.Item{{ID: "one", Title: "Updated"}}}, []string{"one/Updated", "two/Two"}},
		{"append", api.ViewPatch{Mode: api.PatchAppend, Items: []api.Item{{ID: "three", Title: "Three"}}}, []string{"one/Old", "two/Two", "three/Three"}},
		{"prepend", api.ViewPatch{Mode: api.PatchPrepend, Items: []api.Item{{ID: "three", Title: "Three"}}}, []string{"three/Three", "one/Old", "two/Two"}},
		{"remove", api.ViewPatch{RemoveItemIDs: []string{"one"}}, []string{"two/Two"}},
	}
	states := []struct {
		name string
		age  time.Duration
	}{
		{"cold", -1}, {"stale", 2 * time.Minute}, {"expired", 6 * time.Minute},
	}
	for _, state := range states {
		for _, test := range patches {
			t.Run(state.name+"/"+test.name, func(t *testing.T) {
				clock := fake.NewClock()
				cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
				cached := state.age >= 0
				if cached {
					cache.Put("test/key", CacheEntry{Items: []api.Item{{ID: "one", Title: "Old"}, {ID: "two", Title: "Two"}}, At: clock.Now()})
					clock.Advance(state.age)
					cache.puts = nil
				}
				before, _ := cache.Get("test/key")
				var loads []func([]api.Item, *api.SnapshotEvidence, *api.Failure)
				deps := Deps{Cache: cache, Clock: clock, Load: func(_ string, _ string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {
					loads = append(loads, done)
				}, Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
					done(api.Result{Patch: &test.patch}, nil)
				}}
				ref := api.ActionRef{ID: "refresh"}
				view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key"}, Refresh: &api.Refresh{Ref: &ref}}
				s := NewSession(deps, "test", view)
				page := s.Open(&removalHost{})
				s.refreshNow()
				after, exists := cache.Get("test/key")
				if exists != cached || !after.At.Equal(before.At) {
					t.Fatalf("partial refresh renewed snapshot: before=%+v after=%+v exists=%t", before, after, exists)
				}
				if cached && !reflect.DeepEqual(snapshotItemLabels(after.Items), test.want) {
					t.Fatalf("cached items=%v want=%v", snapshotItemLabels(after.Items), test.want)
				}
				writes := len(cache.puts)
				if (!cached || test.name == "status") && writes != 0 {
					t.Fatalf("unexpected cache write: %+v", cache.puts)
				}
				page.OnLeave()
				loads[0]([]api.Item{{ID: "dismissed"}}, nil, nil)
				reopened := NewSession(deps, "test", view)
				reopened.Open(&removalHost{})
				if len(loads) != 2 || !reopened.loading || len(cache.puts) != writes {
					t.Fatalf("reopen skipped source: loads=%d loading=%t writes=%v", len(loads), reopened.loading, cache.puts)
				}
				loads[1]([]api.Item{{ID: "fresh"}}, nil, nil)
				entry, ok := cache.Get("test/key")
				if !ok || len(cache.puts) != writes+1 || len(entry.Items) != 1 || entry.Items[0].ID != "fresh" || !entry.At.Equal(clock.Now()) {
					t.Fatalf("fresh source not stored: entry=%+v writes=%v", entry, cache.puts)
				}
			})
		}
	}
}

func TestPartialRefreshPreservesFreshSnapshotAge(t *testing.T) {
	clock := fake.NewClock()
	cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
	at := clock.Now()
	cache.Put("test/key", CacheEntry{Items: []api.Item{{ID: "one", Title: "Old"}}, At: at})
	clock.Advance(30 * time.Second)
	loads := 0
	deps := Deps{Cache: cache, Clock: clock, Load: func(string, string, func([]api.Item, *api.SnapshotEvidence, *api.Failure)) { loads++ }, Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
		done(api.Result{Patch: &api.ViewPatch{Items: []api.Item{{ID: "one", Title: "Updated"}}}}, nil)
	}}
	ref := api.ActionRef{ID: "refresh"}
	view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key"}, Refresh: &api.Refresh{Ref: &ref}}
	s := NewSession(deps, "test", view)
	page := s.Open(&removalHost{})
	s.refreshNow()
	entry, _ := cache.Get("test/key")
	if !entry.At.Equal(at) || len(entry.Items) != 1 || entry.Items[0].Title != "Updated" {
		t.Fatalf("partial refresh lost snapshot age or row: %+v", entry)
	}
	page.OnLeave()
	clock.Advance(40 * time.Second)
	reopened := NewSession(deps, "test", view)
	reopened.Open(&removalHost{})
	if loads != 1 || !reopened.loading || len(reopened.items) != 1 || reopened.items[0].Title != "Updated" {
		t.Fatalf("reopen did not revalidate preserved rows: loads=%d items=%v", loads, reopened.items)
	}
}

func TestReplacementRefreshOwnsSourceSnapshot(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "items", true: "empty"}[empty], func(t *testing.T) {
			clock := fake.NewClock()
			cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
			items := []api.Item{{ID: "replacement"}}
			if empty {
				items = nil
			}
			var loads []func([]api.Item, *api.SnapshotEvidence, *api.Failure)
			deps := Deps{Cache: cache, Clock: clock, Load: func(_ string, _ string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {
				loads = append(loads, done)
			}, Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
				done(api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: items}}, nil)
			}}
			ref := api.ActionRef{ID: "refresh"}
			view := api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key"}, Refresh: &api.Refresh{Ref: &ref}}
			s := NewSession(deps, "test", view)
			page := s.Open(&removalHost{})
			s.refreshNow()
			loads[0]([]api.Item{{ID: "obsolete"}}, nil, nil)
			if s.loading || len(cache.puts) != 1 || !reflect.DeepEqual(snapshotItemLabels(s.items), snapshotItemLabels(items)) {
				t.Fatalf("obsolete source replaced refresh: loading=%t items=%v writes=%v", s.loading, s.items, cache.puts)
			}
			page.OnLeave()
			reopened := NewSession(deps, "test", view)
			reopened.Open(&removalHost{})
			if len(loads) != 1 || reopened.failure != nil || !reflect.DeepEqual(snapshotItemLabels(reopened.items), snapshotItemLabels(items)) {
				t.Fatalf("replacement not cached: loads=%d items=%v failure=%v", len(loads), reopened.items, reopened.failure)
			}
		})
	}
}

func snapshotItemLabels(items []api.Item) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.ID + "/" + item.Title
	}
	return labels
}
