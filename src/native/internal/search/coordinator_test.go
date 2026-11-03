package search

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// watchdog bounds how long a test waits for a goroutine. It is a guard, never a timing assumption: every
// wait ends as soon as the awaited event happens.
const watchdog = 10 * time.Second

// fixedRows is a provider that answers every call with the same rows.
func fixedRows(id string, items ...Item) Provider {
	return NewRowsProvider(id, func(_ context.Context, _ string) []Item {
		out := make([]Item, len(items))
		copy(out, items)
		return out
	})
}

// newSyncCoordinator returns a coordinator that runs providers inline, so Query returns the complete result.
func newSyncCoordinator(t *testing.T) *Coordinator {
	t.Helper()
	return NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true})
}

// snapshotSink collects the snapshots of an asynchronous coordinator.
type snapshotSink struct{ ch chan Snapshot }

func newSnapshotSink() *snapshotSink { return &snapshotSink{ch: make(chan Snapshot, 256)} }

func (s *snapshotSink) OnSnapshot(snap Snapshot) { s.ch <- snap }

// next waits for the next snapshot, failing the test if none arrives within the watchdog.
func (s *snapshotSink) next(t *testing.T) Snapshot {
	t.Helper()
	select {
	case snap := <-s.ch:
		return snap
	case <-time.After(watchdog):
		t.Fatal("no snapshot arrived")
	}
	return Snapshot{}
}

// untilSettled collects snapshots of generation gen until one is settled, and returns all of them.
func (s *snapshotSink) untilSettled(t *testing.T, gen int) []Snapshot {
	t.Helper()
	var out []Snapshot
	for {
		snap := s.next(t)
		if snap.Gen != gen {
			continue
		}
		out = append(out, snap)
		if snap.Settled {
			return out
		}
	}
}

func titles(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func TestSynchronousQueryMergesByScoreAndSections(t *testing.T) {
	c := newSyncCoordinator(t)
	apps := fixedRows("apps", Item{Kind: KindApplications, Title: "Terminal", Score: 50})
	cmds := fixedRows("commands", Item{Kind: KindCommands, Title: "Clipboard History", Score: 90})
	snap, ready := c.Query("term", []Provider{apps, cmds})
	if !ready || !snap.Settled {
		t.Fatalf("synchronous query: ready=%v settled=%v, want both", ready, snap.Settled)
	}
	if got, want := titles(snap.Items), []string{"Clipboard History", "Terminal"}; !equalStrings(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if snap.Items[0].Section != "Top Hit" || snap.Items[1].Section != KindApplications {
		t.Fatalf("sections = %q, %q; want Top Hit, Applications", snap.Items[0].Section, snap.Items[1].Section)
	}
}

func TestEqualScoresKeepProviderOrder(t *testing.T) {
	c := newSyncCoordinator(t)
	first := fixedRows("first", Item{Kind: KindFiles, Title: "a", Score: 10})
	second := fixedRows("second", Item{Kind: KindFiles, Title: "b", Score: 10})
	snap, _ := c.Query("q", []Provider{first, second})
	if got, want := titles(snap.Items), []string{"a", "b"}; !equalStrings(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestRowsWithTheSameIDAndKindAreMergedOnce(t *testing.T) {
	c := newSyncCoordinator(t)
	a := fixedRows("a", Item{ID: "/x/one", Kind: KindFiles, Title: "one", Score: 5})
	b := fixedRows("b",
		Item{ID: "/x/one", Kind: KindFiles, Title: "one again", Score: 9},
		Item{ID: "/x/one", Kind: KindClipboard, Title: "same id, other kind", Score: 4},
		Item{Kind: KindSnippets, Title: "no id", Score: 3},
		Item{Kind: KindSnippets, Title: "no id either", Score: 2},
	)
	snap, _ := c.Query("one", []Provider{a, b})
	// The first provider's row for an ID and Kind is kept; the rows without an ID are never merged. Sectioning
	// then groups the rest by kind, so the snippets come before the clipboard row.
	if got, want := titles(snap.Items), []string{"one", "no id", "no id either", "same id, other kind"}; !equalStrings(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestMergeKeepsTheLimitOfTheBestRows(t *testing.T) {
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Limit: 2})
	p := fixedRows("p",
		Item{Kind: KindFiles, Title: "low", Score: 1},
		Item{Kind: KindFiles, Title: "high", Score: 9},
		Item{Kind: KindFiles, Title: "mid", Score: 5},
	)
	snap, _ := c.Query("q", []Provider{p})
	if got, want := titles(snap.Items), []string{"high", "mid"}; !equalStrings(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestEmptyQueryAsksForRootRows(t *testing.T) {
	c := newSyncCoordinator(t)
	calls := 0
	root := NewRowsProvider("root", func(_ context.Context, q string) []Item {
		calls++
		if q != "" {
			return []Item{{Title: "searched " + q}}
		}
		return []Item{{Title: "root"}}
	})
	snap, _ := c.Query("", []Provider{root})
	if got := titles(snap.Items); !equalStrings(got, []string{"root"}) {
		t.Fatalf("empty query rows = %v, want the root rows", got)
	}
	// The coordinator takes the query as given: a blank query is a search, and the palette trims before it asks.
	snap, _ = c.Query("  ", []Provider{root})
	if calls != 2 || len(snap.Items) != 1 || snap.Items[0].Title != "searched   " {
		t.Fatalf("blank query: root calls = %d, rows = %v", calls, titles(snap.Items))
	}
}

func TestCacheReturnsTheRowsOfAQueryAtOnce(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot})
	p := fixedRows("p", Item{Kind: KindFiles, Title: "remembered", Score: 1})
	if _, ready := c.Query("rem", []Provider{p}); ready {
		t.Fatal("a first query has nothing ready yet")
	}
	sink.untilSettled(t, c.Gen())
	again, ready := c.Query("rem", []Provider{p})
	if !ready || !equalStrings(titles(again.Items), []string{"remembered"}) {
		t.Fatalf("repeated query: ready=%v rows=%v, want the remembered rows at once", ready, titles(again.Items))
	}
	c.Invalidate()
	if _, ready := c.Query("rem", []Provider{p}); ready {
		t.Fatal("after Invalidate nothing is remembered")
	}
}

func TestStaleBatchesOfAnOlderQueryAreDropped(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot})
	release := make(chan struct{})
	slow := NewRowsProvider("slow", func(ctx context.Context, q string) []Item {
		if q == "first" {
			<-release
			return []Item{{Title: "stale rows for first"}}
		}
		return []Item{{Title: "rows for " + q}}
	})
	c.Query("first", []Provider{slow})
	firstGen := c.Gen()
	c.Query("second", []Provider{slow})
	close(release)
	snaps := sink.untilSettled(t, c.Gen())
	for _, snap := range snaps {
		if snap.Gen == firstGen {
			t.Fatal("a snapshot of the older query was delivered")
		}
		for _, it := range snap.Items {
			if it.Title == "stale rows for first" {
				t.Fatal("stale rows reached a newer query")
			}
		}
	}
	if !equalStrings(titles(snaps[len(snaps)-1].Items), []string{"rows for second"}) {
		t.Fatalf("settled rows = %v", titles(snaps[len(snaps)-1].Items))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
