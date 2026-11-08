package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestManualRefreshPreservesRowsAndRejectsObsoleteLoads(t *testing.T) {
	clock := fake.NewClock()
	cache := NewMemoryCache()
	cache.Put("test/key", CacheEntry{Items: []api.Item{{ID: "cached"}}, At: clock.Now()})
	var loads []func([]api.Item, *api.Failure)
	s := NewSession(Deps{Clock: clock, Cache: cache, Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }}, "test", api.View{Kind: api.ViewList, Source: &api.DataSource{CacheKey: "key", Loader: "load"}})
	host := &removalHost{}
	page := s.Open(host)
	action := s.actionBody(api.Action{Kind: api.ActionRefreshView}, "refresh")
	action.Run(host)
	action.Run(host)
	if len(loads) != 2 || len(s.items) != 1 || s.items[0].ID != "cached" {
		t.Fatalf("loads=%d items=%v", len(loads), s.items)
	}
	loads[1]([]api.Item{{ID: "new"}}, nil)
	loads[0]([]api.Item{{ID: "old"}}, nil)
	if s.items[0].ID != "new" {
		t.Fatalf("obsolete load applied: %v", s.items)
	}
	entry, _ := cache.Get("test/key")
	if entry.Items[0].ID != "new" {
		t.Fatalf("obsolete snapshot applied: %v", entry.Items)
	}
	action.Run(host)
	page.OnLeave()
	loads[2]([]api.Item{{ID: "dismissed"}}, nil)
	if s.items[0].ID != "new" {
		t.Fatalf("dismissed load applied: %v", s.items)
	}
}

func TestManualRefreshReportsUnavailable(t *testing.T) {
	s := NewSession(Deps{}, "test", api.View{})
	h := &fakeHost{}
	s.actionBody(api.Action{Kind: api.ActionRefreshView}, "").Run(h)
	if len(h.toasts) != 1 || h.toasts[0] != unavailableMessage {
		t.Fatalf("toasts: %v", h.toasts)
	}
}

func TestManualRefreshJoinsInFlightAndReportsFailure(t *testing.T) {
	var done func(api.Result, *api.Failure)
	calls := 0
	ref := api.ActionRef{ID: "refresh"}
	s := NewSession(Deps{Invoke: func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { calls++; done = cb }}, "test", api.View{Kind: api.ViewList, Refresh: &api.Refresh{Ref: &ref}})
	h := &fakeHost{}
	s.refreshNow()
	action := s.actionBody(api.Action{Kind: api.ActionRefreshView}, "")
	action.Run(h)
	action.Run(h)
	if calls != 1 {
		t.Fatalf("calls: %d", calls)
	}
	done(api.Result{}, &api.Failure{Message: "refresh failed"})
	if len(h.toasts) != 1 || h.toasts[0] != "refresh failed" {
		t.Fatalf("toasts: %v", h.toasts)
	}
}

func TestManualRefreshRejectsIncompatibleViews(t *testing.T) {
	ref := &api.ActionRef{ID: "search"}
	source := &api.DataSource{Loader: "load"}
	handler := &api.Refresh{Ref: &api.ActionRef{ID: "refresh"}}
	views := []api.View{
		{Kind: api.ViewPreview, Source: source},
		{Kind: api.ViewEditor, Refresh: handler},
		{Kind: api.ViewList, Source: source, Search: ref},
		{Kind: api.ViewGrid, Source: source, Refresh: handler},
		{Kind: api.ViewList, Refresh: &api.Refresh{Ref: &api.ActionRef{}}},
	}
	for _, view := range views {
		calls := 0
		s := NewSession(Deps{Load: func(string, string, func([]api.Item, *api.Failure)) { calls++ }, Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) { calls++ }}, "test", view)
		h := &fakeHost{}
		s.actionBody(api.Action{Kind: api.ActionRefreshView}, "").Run(h)
		if calls != 0 || len(h.toasts) != 1 || h.toasts[0] != unavailableMessage {
			t.Fatalf("view=%v calls=%d toasts=%v", view.Kind, calls, h.toasts)
		}
	}
}

func TestManualSourceRefreshKeepsRowsOnFailure(t *testing.T) {
	var done func([]api.Item, *api.Failure)
	s := NewSession(Deps{Load: func(_ string, _ string, cb func([]api.Item, *api.Failure)) { done = cb }}, "test", api.View{Kind: api.ViewGrid, Source: &api.DataSource{Loader: "load"}})
	s.items = []api.Item{{ID: "cached"}}
	h := &fakeHost{}
	s.actionBody(api.Action{Kind: api.ActionRefreshView}, "").Run(h)
	done(nil, &api.Failure{Message: "load failed"})
	if len(s.items) != 1 || s.items[0].ID != "cached" || s.failure == nil || s.failure.Message != "load failed" {
		t.Fatalf("items=%v failure=%v", s.items, s.failure)
	}
}
