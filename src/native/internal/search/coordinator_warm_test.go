package search

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type refreshControl struct{ calls chan chan Batch }

func (p refreshControl) ID() string                                       { return "apps" }
func (p refreshControl) Root(context.Context) []Item                      { return nil }
func (p refreshControl) RootStream(ctx context.Context, emit func(Batch)) { p.Search(ctx, "", emit) }
func (p refreshControl) Search(ctx context.Context, _ string, emit func(Batch)) {
	batches := make(chan Batch)
	select {
	case p.calls <- batches:
	case <-ctx.Done():
		return
	}
	for {
		select {
		case b := <-batches:
			emit(b)
			if b.Final {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func TestWarmProviderRowsSurviveOtherProviderRefresh(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true})
	t.Cleanup(c.Cancel)
	app := refreshControl{calls: make(chan chan Batch, 2)}
	commands := fixedRows("commands", Item{ID: "settings", Kind: KindCommands, Title: "Settings", Score: 1})
	providers := []Provider{app, commands}
	c.Query("", providers)
	first := <-app.calls
	first <- Batch{Items: []Item{{ID: "old-app", Kind: KindApplications, Title: "Old app", Score: 10}}, Final: true}
	sink.untilSettled(t, c.Gen())
	warm, ready := c.Query("", providers)
	if !ready || !containsTitle(warm.Items, "Old app") {
		t.Fatal("warm opening lost its cached app")
	}
	second := <-app.calls
	// Only the command provider can answer now: apps must remain in every partial snapshot.
	for {
		snap := sink.next(t)
		if snap.Gen != c.Gen() {
			continue
		}
		if !containsTitle(snap.Items, "Old app") || !containsTitle(snap.Items, "Settings") {
			t.Fatalf("partial refresh lost apps: %v", titles(snap.Items))
		}
		break
	}
	second <- Batch{Items: []Item{{ID: "new-app", Kind: KindApplications, Title: "New app", Score: 10}}}
	fresh := sink.next(t)
	if containsTitle(fresh.Items, "Old app") || !containsTitle(fresh.Items, "New app") {
		t.Fatalf("first fresh app batch did not replace its stale rows: %v", titles(fresh.Items))
	}
	second <- Batch{Replace: true, Final: true}
	settled := sink.untilSettled(t, c.Gen())
	if rows := settled[len(settled)-1].Items; containsTitle(rows, "Old app") || containsTitle(rows, "New app") {
		t.Fatalf("empty authoritative answer kept stale apps: %v", titles(rows))
	}
}

func TestWarmProviderCacheIsBoundedByVisibleRows(t *testing.T) {
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Limit: 1, Flat: true})
	app := fixedRows("apps", Item{ID: "low", Kind: KindApplications, Title: "Low", Score: 1}, Item{ID: "best", Kind: KindApplications, Title: "Best", Score: 100})
	c.Query("", []Provider{app})
	cached, _ := c.cache.get("")
	if rows := cached.providers["apps"]; len(rows) != 1 || rows[0].ID != "best" {
		t.Fatalf("warm cache retained hidden inventory: %+v", rows)
	}
}

func TestFailedWarmRefreshKeepsItsCachedRows(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot})
	t.Cleanup(c.Cancel)
	calls := 0
	app := NewRowsProvider("apps", func(context.Context, string) []Item {
		calls++
		if calls > 1 {
			panic("inventory temporarily unavailable")
		}
		return []Item{{ID: "app", Kind: KindApplications, Title: "Cached app", Score: 10}}
	})
	c.Query("", []Provider{app})
	sink.untilSettled(t, c.Gen())
	c.Query("", []Provider{app})
	settled := sink.untilSettled(t, c.Gen())
	if rows := settled[len(settled)-1].Items; !containsTitle(rows, "Cached app") {
		t.Fatalf("failed revalidation erased cached apps: %v", titles(rows))
	}
}
