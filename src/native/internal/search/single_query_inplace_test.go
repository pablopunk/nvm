package search

import (
	"context"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func rowAccessories(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID+":"+item.Accessory)
	}
	return out
}

// TestFixedQueryRefreshUpdatesShownRowsInPlace refreshes the typed query on screen. A status that arrived later shows
// on its row; the rows keep their order, a row the refresh adds waits for the next query, and a refresh that draws
// the same rows publishes nothing.
func TestFixedQueryRefreshUpdatesShownRowsInPlace(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, SingleSnapshot: true})
	defer c.Cancel()
	p := singleProvider{id: "apps", calls: make(chan chan singleAnswer, 3)}
	c.Query("fake", []Provider{p})
	answerSingle(<-p.calls, []Item{{ID: "a", Title: "Fake app", Score: 2}, {ID: "b", Title: "Fake tool", Score: 1}}, true)
	first := sink.untilSettled(t, c.Gen())
	publication := first[len(first)-1].Items[0].Publication

	shown, ready := c.Query("fake", []Provider{p})
	if !ready || !slices.Equal(rowIDs(shown.Items), []string{"a", "b"}) {
		t.Fatalf("refresh did not keep the rows on screen: %v", rowIDs(shown.Items))
	}
	answerSingle(<-p.calls, []Item{{ID: "c", Title: "Fake new", Score: 9}, {ID: "b", Title: "Fake tool", Score: 5}, {ID: "a", Title: "Fake app", Accessory: "Running", Score: 1}}, true)
	updated := sink.untilSettled(t, c.Gen())
	rows := updated[len(updated)-1].Items
	if got := rowAccessories(rows); !slices.Equal(got, []string{"a:Running", "b:"}) {
		t.Fatalf("updated rows = %v, want the same rows in order with the status", got)
	}
	if rows[0].Publication != publication {
		t.Fatal("the updated row lost the publication its actions are bound to")
	}

	c.Query("fake", []Provider{p})
	answerSingle(<-p.calls, []Item{{ID: "a", Title: "Fake app", Accessory: "Running"}, {ID: "b", Title: "Fake tool"}}, true)
	assertNoSnapshot(t, sink)
}

// TestExactCacheHitUpdatesItsRowsInPlace returns to a remembered query. Its remembered rows show at once and keep their
// set; the refresh brings the new title of the same row.
func TestExactCacheHitUpdatesItsRowsInPlace(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, SingleSnapshot: true})
	defer c.Cancel()
	p := singleProvider{id: "fixture", calls: make(chan chan singleAnswer, 3)}
	c.Query("history", []Provider{p})
	answerSingle(<-p.calls, []Item{{ID: "history", Title: "history entries=1"}}, true)
	sink.untilSettled(t, c.Gen())
	c.Query("other", []Provider{p})
	answerSingle(<-p.calls, []Item{{ID: "other", Title: "Other"}}, true)
	sink.untilSettled(t, c.Gen())

	warm, ready := c.Query("history", []Provider{p})
	if !ready || len(warm.Items) != 1 || warm.Items[0].Title != "history entries=1" {
		t.Fatalf("warm rows = %+v", warm.Items)
	}
	answerSingle(<-p.calls, []Item{{ID: "history", Title: "history entries=0"}, {ID: "late", Title: "history late"}}, true)
	refreshed := sink.untilSettled(t, c.Gen())
	if got := refreshed[len(refreshed)-1].Items; len(got) != 1 || got[0].Title != "history entries=0" {
		t.Fatalf("refreshed rows = %v", titles(got))
	}
}

// TestEmptyQueryListsTheCatalogAsItArrives opens the root before one of its sources answered. The empty query is the
// root's catalog, not a typed result, so the late source's rows join it.
func TestEmptyQueryListsTheCatalogAsItArrives(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot, Flat: true, SingleSnapshot: true})
	defer c.Cancel()
	late := scriptedRoot{singleProvider{id: "apps", calls: make(chan chan singleAnswer, 1)}}
	commands := fixedRows("commands", Item{ID: "settings", Title: "Settings", Score: 2})
	c.Query("", []Provider{late, commands})
	call := <-late.calls
	passFirstSnapshot(t, c, clock, "apps")
	if first := sink.next(t); !slices.Equal(rowIDs(first.Items), []string{"settings"}) {
		t.Fatalf("first root = %v", rowIDs(first.Items))
	}
	answerSingle(call, []Item{{ID: "loom", Title: "Loom", Score: 1}}, true)
	last := sink.untilSettled(t, c.Gen())
	if got := rowIDs(last[len(last)-1].Items); !slices.Equal(got, []string{"settings", "loom"}) {
		t.Fatalf("root after the apps answered = %v", got)
	}
}

// scriptedRoot answers the empty query the way singleProvider answers a typed one.
type scriptedRoot struct{ singleProvider }

func (p scriptedRoot) RootStream(ctx context.Context, emit func(Batch)) { p.Search(ctx, "", emit) }
