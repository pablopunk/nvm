package extview

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type removalChats struct {
	ChatPages
	rows []api.ChatSummary
	fail bool
}

type removalAPIChats struct {
	api.AIChats
	chats *removalChats
}

func (c removalAPIChats) List() []api.ChatSummary { return c.chats.rows }
func (c *removalChats) Remove(id string) error {
	if c.fail {
		return errors.New("Could not remove chat")
	}
	for i, row := range c.rows {
		if row.ID == id {
			c.rows = append(c.rows[:i], c.rows[i+1:]...)
			break
		}
	}
	return nil
}

type removalAI struct {
	api.AI
	chats *removalChats
}

func (a removalAI) Chats() api.AIChats { return removalAPIChats{chats: a.chats} }

type removalClock struct{ api.Clock }

func (removalClock) Now() time.Time { return time.Unix(100, 0) }

type removalNavigation struct{ api.Navigation }

func (removalNavigation) Push(v *api.View) api.Result { return api.Result{View: v} }

type removalContext struct {
	api.Context
	chats *removalChats
}

func (c removalContext) AI() api.AI               { return removalAI{chats: c.chats} }
func (removalContext) Clock() api.Clock           { return removalClock{} }
func (removalContext) Navigation() api.Navigation { return removalNavigation{} }

func TestChatRemovalRefreshesOpenHistory(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			chats := &removalChats{rows: []api.ChatSummary{{ID: "one", Kind: "conversation", Title: "One"}, {ID: "two", Kind: "builder", Title: "Two"}}, fail: fail}
			ctx := removalContext{chats: chats}
			ext := aichats.New()
			initial, err := ext.RunAction(ctx, api.Invocation{Ref: aichats.HistoryRef()})
			if err != nil {
				t.Fatal(err)
			}
			calls, foreground := 0, 0
			deps := Deps{Chats: chats, Invoke: func(id string, in api.Invocation, done func(api.Result, *api.Failure)) {
				calls++
				result, err := ext.RunAction(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				done(result, nil)
			}, InvokeCommand: func(string, api.Invocation, func(api.Result, *api.Failure)) { foreground++ }}
			s := NewSession(deps, aichats.ID, *initial.View)
			host := &fakeHost{}
			s.Open(host)
			action := s.items[1].Actions[1]
			s.actionBody(action, "").Run(host)
			if fail {
				if len(chats.rows) != 2 || len(s.items) != 3 || calls != 0 || len(host.toasts) != 1 || host.toasts[0] != "Could not remove chat" {
					t.Fatalf("failure: rows=%v items=%v calls=%d toast=%v", chats.rows, s.items, calls, host.toasts)
				}
				return
			}
			if len(chats.rows) != 1 || chats.rows[0].ID != "two" || len(s.items) != 2 || s.items[1].ID != "two" || calls != 1 || foreground != 0 {
				t.Fatalf("success: rows=%v items=%v calls=%d foreground=%d", chats.rows, s.items, calls, foreground)
			}
		})
	}
}

type removalHost struct {
	fakeHost
	query string
	sets  int
}

func (h *removalHost) Query() string     { return h.query }
func (h *removalHost) SetQuery(q string) { h.query = q; h.sets++ }

type recordingSnapshotCache struct {
	Cache
	puts []CacheEntry
}

func (c *recordingSnapshotCache) Put(key string, entry CacheEntry)  { testCachePut(c, key, entry) }
func (c *recordingSnapshotCache) Get(key string) (CacheEntry, bool) { return testCacheGet(c, key) }
func (c *recordingSnapshotCache) acquire(id string, identity any) *sourceLease {
	lease := c.Cache.acquire(id, identity)
	lease.onPut = func(entry CacheEntry) {
		entry.Items = append([]api.Item(nil), entry.Items...)
		c.puts = append(c.puts, entry)
	}
	return lease
}

func TestChatRemovalDuringHistoryRefresh(t *testing.T) {
	chats := &removalChats{rows: []api.ChatSummary{{ID: "one", Kind: "conversation", Title: "One"}, {ID: "two", Kind: "builder", Title: "Two"}}}
	ctx := removalContext{chats: chats}
	ext := aichats.New()
	initial, err := ext.RunAction(ctx, api.Invocation{Ref: aichats.HistoryRef()})
	if err != nil {
		t.Fatal(err)
	}
	initial.View.Source = &api.DataSource{CacheKey: "history"}
	cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
	clock := fake.NewClock()
	cache.Put(aichats.ID+"/history", CacheEntry{Items: initial.View.Items, At: clock.Now()})
	cache.puts = nil
	var old func(api.Result, *api.Failure)
	var oldResult api.Result
	calls := 0
	deps := Deps{Chats: chats, Cache: cache, Clock: clock, Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		calls++
		result, err := ext.RunAction(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			old, oldResult = done, result
			return
		}
		done(result, nil)
	}}
	s := NewSession(deps, aichats.ID, *initial.View)
	host := &removalHost{query: "two"}
	s.Open(host)
	s.refreshNow()
	s.actionBody(s.items[1].Actions[1], "").Run(host)
	if calls != 1 {
		t.Fatalf("refresh started while busy: %d", calls)
	}
	old(oldResult, nil)
	if len(cache.puts) != 1 || len(cache.puts[0].Items) != 2 || cache.puts[0].Items[1].ID != "two" {
		t.Fatalf("cache writes after deletion: %+v", cache.puts)
	}
	if calls != 2 || len(s.items) != 2 || s.items[1].ID != "two" {
		t.Fatalf("calls=%d items=%v", calls, s.items)
	}
	if host.query != "two" || host.sets != 0 || host.pops != 0 || len(host.pushed) != 0 {
		t.Fatalf("navigation changed: %+v", host)
	}
	entry, ok := cache.Get(aichats.ID + "/history")
	if !ok || len(entry.Items) != 2 || entry.Items[1].ID != "two" {
		t.Fatalf("cache=%v", entry.Items)
	}
	s.close()
	reopened := NewSession(deps, aichats.ID, *initial.View)
	reopened.Open(&removalHost{})
	if len(reopened.items) != 2 || reopened.items[1].ID != "two" {
		t.Fatalf("reopened=%v", reopened.items)
	}
}

func TestChatRemovalRefreshDismissed(t *testing.T) {
	chats := &removalChats{rows: []api.ChatSummary{{ID: "one", Kind: "conversation", Title: "One"}, {ID: "two", Kind: "builder", Title: "Two"}}}
	ctx := removalContext{chats: chats}
	ext := aichats.New()
	initial, err := ext.RunAction(ctx, api.Invocation{Ref: aichats.HistoryRef()})
	if err != nil {
		t.Fatal(err)
	}
	initial.View.Source = &api.DataSource{CacheKey: "history"}
	cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
	clock := fake.NewClock()
	cache.Put(aichats.ID+"/history", CacheEntry{Items: initial.View.Items, At: clock.Now()})
	cache.puts = nil
	var callback func(api.Result, *api.Failure)
	var result api.Result
	deps := Deps{Chats: chats, Cache: cache, Clock: clock, Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		var err error
		result, err = ext.RunAction(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		callback = done
	}}
	s := NewSession(deps, aichats.ID, *initial.View)
	host := &removalHost{}
	page := s.Open(host)
	s.actionBody(s.items[1].Actions[1], "").Run(host)
	if callback == nil {
		t.Fatal("refresh callback was not held")
	}
	beforeItems := append([]api.Item(nil), s.items...)
	beforeRefresh := host.refresh
	page.OnLeave()
	callback(result, nil)
	if len(cache.puts) != 0 || host.refresh != beforeRefresh || len(s.items) != len(beforeItems) {
		t.Fatalf("dismissed callback: puts=%v refresh=%d items=%v", cache.puts, host.refresh-beforeRefresh, s.items)
	}
	for i := range beforeItems {
		if s.items[i].ID != beforeItems[i].ID {
			t.Fatalf("dismissed callback changed items: %v", s.items)
		}
	}
}

func TestRemovalInvalidatesHeldSource(t *testing.T) {
	for _, declared := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "refresh"}[declared], func(t *testing.T) {
			chats := &removalChats{rows: []api.ChatSummary{{ID: "one", Kind: "conversation", Title: "One"}, {ID: "two", Kind: "builder", Title: "Two"}}}
			ctx := removalContext{chats: chats}
			ext := aichats.New()
			initial, err := ext.RunAction(ctx, api.Invocation{Ref: aichats.HistoryRef()})
			if err != nil {
				t.Fatal(err)
			}
			initial.View.Source = &api.DataSource{CacheKey: "history"}
			if !declared {
				initial.View.Refresh = nil
			}
			cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
			var loads []func([]api.Item, *api.Failure)
			deps := Deps{Chats: chats, Cache: cache, Clock: fake.NewClock(), Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }, Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
				result, err := ext.RunAction(ctx, in)
				if err != nil {
					t.Fatal(err)
				}
				done(result, nil)
			}}
			s := NewSession(deps, aichats.ID, *initial.View)
			host := &removalHost{}
			s.Open(host)
			old := append([]api.Item(nil), initial.View.Items...)
			s.actionBody(s.items[1].Actions[1], "").Run(host)
			if !declared {
				if len(loads) != 1 {
					t.Fatalf("loads=%d", len(loads))
				}
				loads[0](old, nil)
				if len(loads) != 2 {
					t.Fatalf("coalesced loads=%d", len(loads))
				}
				fresh, err := ext.RunAction(ctx, api.Invocation{Ref: aichats.HistoryRef()})
				if err != nil {
					t.Fatal(err)
				}
				loads[1](fresh.View.Items, nil)
			}
			before := len(cache.puts)
			if before == 0 {
				t.Fatal("fresh result was not cached")
			}
			if declared {
				loads[0](old, nil)
			}
			if len(cache.puts) != before || len(s.items) != 2 || s.items[1].ID != "two" {
				t.Fatalf("stale source applied: items=%v puts=%v", s.items, cache.puts)
			}
		})
	}
}

func TestSourceLatestAndDismissed(t *testing.T) {
	cache := &recordingSnapshotCache{Cache: NewMemoryCache()}
	var loads []func([]api.Item, *api.Failure)
	s := NewSession(Deps{Cache: cache, Clock: fake.NewClock(), Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { loads = append(loads, done) }}, "test", api.View{Source: &api.DataSource{CacheKey: "key"}})
	s.Open(&removalHost{})
	s.reload()
	loads[0]([]api.Item{{ID: "old"}}, nil)
	loads[1]([]api.Item{{ID: "new"}}, nil)
	if len(cache.puts) != 1 || len(s.items) != 1 || s.items[0].ID != "new" {
		t.Fatalf("latest load lost: %v %v", s.items, cache.puts)
	}
	s.reload()
	s.close()
	loads[2]([]api.Item{{ID: "closed"}}, nil)
	if len(cache.puts) != 1 || s.items[0].ID != "new" {
		t.Fatalf("dismissed load applied: %v %v", s.items, cache.puts)
	}
}

func TestStatusRefreshPreservesColdSource(t *testing.T) {
	var load func([]api.Item, *api.Failure)
	ref := api.ActionRef{ID: "refresh"}
	s := NewSession(Deps{Load: func(_ string, _ string, done func([]api.Item, *api.Failure)) { load = done }, Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
		done(api.Result{Patch: &api.ViewPatch{Status: "ready"}}, nil)
	}}, "test", api.View{Source: &api.DataSource{}, Refresh: &api.Refresh{Ref: &ref}})
	s.Open(&removalHost{})
	s.refreshNow()
	load([]api.Item{{ID: "loaded"}}, nil)
	if s.view.Status != "ready" || len(s.items) != 1 || s.items[0].ID != "loaded" {
		t.Fatalf("status=%s items=%v", s.view.Status, s.items)
	}
}
