package search

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

type singleAnswer struct {
	rows         []Item
	final        bool
	acknowledged chan struct{}
}
type singleProvider struct {
	id    string
	calls chan chan singleAnswer
}

func (p singleProvider) ID() string                  { return p.id }
func (p singleProvider) Root(context.Context) []Item { return nil }
func (p singleProvider) Search(ctx context.Context, _ string, emit func(Batch)) {
	answers := make(chan singleAnswer)
	select {
	case p.calls <- answers:
	case <-ctx.Done():
		return
	}
	for {
		select {
		case a := <-answers:
			emit(Batch{Items: a.rows, Final: a.final})
			close(a.acknowledged)
			if a.final {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
func answerSingle(ch chan singleAnswer, rows []Item, final bool) {
	ack := make(chan struct{})
	ch <- singleAnswer{rows: rows, final: final, acknowledged: ack}
	<-ack
}
func assertNoSnapshot(t *testing.T, sink *snapshotSink) {
	t.Helper()
	select {
	case got := <-sink.ch:
		t.Fatalf("unexpected partial snapshot: %+v", got)
	default:
	}
}

func TestSingleSnapshotWaitsForEveryProviderRegardlessOfOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		sink := newSnapshotSink()
		c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, Tiered: true, SingleSnapshot: true})
		a := singleProvider{id: "a", calls: make(chan chan singleAnswer, 1)}
		b := singleProvider{id: "b", calls: make(chan chan singleAnswer, 1)}
		if _, ready := c.Query("loo", []Provider{a, b}); ready {
			t.Fatal("cold partial answer was ready")
		}
		ca, cb := <-a.calls, <-b.calls
		first, second := ca, cb
		firstRows, secondRows := []Item{{ID: "weak", Title: "Balloon"}}, []Item{{ID: "loom", Title: "Loom"}, {ID: "web", Title: "Search web", QueryFallback: true}}
		if reverse {
			first, second = cb, ca
			firstRows, secondRows = secondRows, firstRows
		}
		answerSingle(first, firstRows, true)
		assertNoSnapshot(t, sink)
		answerSingle(second, secondRows, true)
		snap := sink.untilSettled(t, c.Gen())
		got := rowIDs(snap[len(snap)-1].Items)
		if !slices.Equal(got, []string{"loom", "weak", "web"}) {
			t.Fatalf("order=%v", got)
		}
		again, ready := c.Query("loo", []Provider{a, b})
		if !ready || !slices.Equal(rowIDs(again.Items), got) {
			t.Fatalf("same query changed: %v", rowIDs(again.Items))
		}
		assertNoSnapshot(t, sink)
		c.Cancel()
	}
}

func TestSingleSnapshotKeepsLeadingMatchDuringNarrowing(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, Tiered: true, SingleSnapshot: true})
	defer c.Cancel()
	p := singleProvider{id: "apps", calls: make(chan chan singleAnswer, 4)}
	c.Query("lo", []Provider{p})
	first := <-p.calls
	answerSingle(first, []Item{{ID: "loom", Title: "Loom", Kind: KindApplications}}, true)
	sink.untilSettled(t, c.Gen())
	c.Query("loo", []Provider{p})
	next := <-p.calls
	answerSingle(next, []Item{{ID: "loo", Title: "Loo", Kind: KindCommands}, {ID: "loom", Title: "Loom", Kind: KindApplications}}, true)
	snaps := sink.untilSettled(t, c.Gen())
	if got := rowIDs(snaps[len(snaps)-1].Items); !slices.Equal(got, []string{"loom", "loo"}) {
		t.Fatalf("narrowed = %v", got)
	}
}

func TestSingleSnapshotWarmRefreshIsNotPublished(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, SingleSnapshot: true})
	defer c.Cancel()
	p := singleProvider{id: "p", calls: make(chan chan singleAnswer, 4)}
	c.Query("a", []Provider{p})
	a := <-p.calls
	answerSingle(a, []Item{{ID: "old", Title: "Old"}}, true)
	sink.untilSettled(t, c.Gen())
	c.Query("b", []Provider{p})
	b := <-p.calls
	answerSingle(b, []Item{{ID: "b", Title: "B"}}, true)
	sink.untilSettled(t, c.Gen())
	warm, ok := c.Query("a", []Provider{p})
	if !ok || !slices.Equal(rowIDs(warm.Items), []string{"old"}) {
		t.Fatalf("warm=%v", rowIDs(warm.Items))
	}
	refreshed := <-p.calls
	answerSingle(refreshed, []Item{{ID: "new", Title: "New"}}, true)
	assertNoSnapshot(t, sink)
	same, ok := c.Query("a", []Provider{p})
	if !ok || !slices.Equal(rowIDs(same.Items), []string{"old"}) {
		t.Fatalf("refresh changed visible rows: %v", rowIDs(same.Items))
	}
}

func TestSingleSnapshotRejectsSupersededGeneration(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, SingleSnapshot: true})
	defer c.Cancel()
	p := singleProvider{id: "p", calls: make(chan chan singleAnswer, 4)}
	c.Query("old", []Provider{p})
	old := c.Gen()
	<-p.calls
	c.Query("new", []Provider{p})
	next := <-p.calls
	c.deliver(old, 0, Batch{Items: []Item{{ID: "stale"}}, Final: true})
	assertNoSnapshot(t, sink)
	answerSingle(next, []Item{{ID: "new"}}, true)
	got := sink.untilSettled(t, c.Gen())
	if !slices.Equal(rowIDs(got[len(got)-1].Items), []string{"new"}) {
		t.Fatal("stale generation leaked")
	}
}

func TestCachedNarrowingPreservesPreviousLeader(t *testing.T) {
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, Flat: true, Tiered: true, SingleSnapshot: true, Limit: 1})
	p := NewRowsProvider("apps", func(_ context.Context, q string) []Item {
		if q == "loo" {
			return []Item{{ID: "loo", Title: "Loo", Kind: KindCommands}, {ID: "loom", Title: "Loom", Kind: KindApplications}}
		}
		return []Item{{ID: "loom", Title: "Loom", Kind: KindApplications}}
	})
	first, _ := c.Query("loo", []Provider{p})
	if rowIDs(first.Items)[0] != "loo" {
		t.Fatal("setup did not cache competing exact match")
	}
	c.Query("lo", []Provider{p})
	// Warm cache is ranked afresh around the previously displayed leader.
	c.opts.Synchronous = false
	warm, ok := c.Query("loo", []Provider{p})
	if !ok || len(warm.Items) != 1 || warm.Items[0].ID != "loom" {
		t.Fatalf("cached narrowing = %v", rowIDs(warm.Items))
	}
	c.Cancel()
}

type selfBudgetedSingle struct{ singleProvider }

func (selfBudgetedSingle) OwnsCallBudgets() {}

func TestSingleSnapshotDoesNotPublishPartialSelfBudgetedAggregate(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, SingleSnapshot: true})
	defer c.Cancel()
	p := selfBudgetedSingle{singleProvider{id: "aggregate", calls: make(chan chan singleAnswer, 1)}}
	c.Query("q", []Provider{p})
	call := <-p.calls
	answerSingle(call, []Item{{ID: "early", Score: 1}}, false)
	c.cutOff(c.Gen(), 0)
	assertNoSnapshot(t, sink)
	answerSingle(call, []Item{{ID: "late", Score: 2}}, true)
	snaps := sink.untilSettled(t, c.Gen())
	if got := rowIDs(snaps[len(snaps)-1].Items); !slices.Equal(got, []string{"late", "early"}) {
		t.Fatalf("complete aggregate = %v", got)
	}
}

type admissibleSeed struct {
	singleProvider
	allowed *atomic.Bool
}

func (p admissibleSeed) SeedNarrowing(q string, old []Item) []Item {
	if p.allowed.Load() && len(old) == 1 && MatchesName(q, old[0].Title) {
		return old
	}
	return nil
}

func TestNarrowingRetainsMatchingLeaderOutsideProviderLimitUnlessRevoked(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		sink := newSnapshotSink()
		c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot, Flat: true, Tiered: true, SingleSnapshot: true})
		var permission atomic.Bool
		permission.Store(true)
		p := admissibleSeed{singleProvider: singleProvider{id: "apps", calls: make(chan chan singleAnswer, 2)}, allowed: &permission}
		c.Query("lo", []Provider{p})
		first := <-p.calls
		answerSingle(first, []Item{{ID: "loom", Title: "Loom", Kind: KindApplications}}, true)
		sink.untilSettled(t, c.Gen())
		c.Query("loo", []Provider{p})
		next := <-p.calls
		permission.Store(allowed)
		answerSingle(next, []Item{{ID: "loo", Title: "Loo", Kind: KindCommands}}, true)
		snapshots := sink.untilSettled(t, c.Gen())
		rows := snapshots[len(snapshots)-1].Items
		if allowed && (len(rows) == 0 || rows[0].ID != "loom") {
			t.Fatalf("matching leader lost: %v", rowIDs(rows))
		}
		if !allowed && containsTitle(rows, "Loom") {
			t.Fatal("revoked leader survived")
		}
		c.Cancel()
	}
}
