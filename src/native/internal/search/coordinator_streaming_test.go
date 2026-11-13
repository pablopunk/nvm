package search

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// gatedProvider answers a search with its first row at once, and with its late row only once release is closed. It is
// the shape of an extension that answers late: its rows arrive after the hard budget has cut its call off.
type gatedProvider struct {
	id      string
	first   Item
	late    Item
	release <-chan struct{}
}

func (p gatedProvider) OwnsCallBudgets() {}

func (p gatedProvider) ID() string                  { return p.id }
func (p gatedProvider) Root(context.Context) []Item { return nil }

func (p gatedProvider) Search(_ context.Context, _ string, emit func(Batch)) {
	emit(Batch{Items: []Item{p.first}})
	<-p.release
	emit(Batch{Items: []Item{p.late}, Final: true})
}

// streamedRoot is a root provider whose sources answer one batch at a time: the first at once, the second when release
// is closed.
type streamedRoot struct {
	first, second Item
	release       <-chan struct{}
}

func (p streamedRoot) ID() string                                  { return "streamed" }
func (p streamedRoot) Root(ctx context.Context) []Item             { return nil }
func (p streamedRoot) Search(context.Context, string, func(Batch)) {}

func (p streamedRoot) RootStream(_ context.Context, emit func(Batch)) {
	emit(Batch{Items: []Item{p.first}})
	<-p.release
	emit(Batch{Items: []Item{p.second}, Final: true})
}

// TestLateRowsOfACutOffProviderMergeIntoItsQuery checks that a provider cut off at the hard budget is counted as slow,
// the query settles without it, and the rows it emits afterwards still merge into the same query.
func TestLateRowsOfACutOffProviderMergeIntoItsQuery(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot})
	release := make(chan struct{})
	slow := gatedProvider{id: "slow", first: Item{Kind: KindFiles, Title: "early row", Score: 1}, late: Item{Kind: KindFiles, Title: "late row", Score: 2}, release: release}
	c.Query("q", []Provider{slow})
	gen := c.Gen()
	passFirstSnapshot(t, c, clock)

	if first := sink.next(t); first.Settled || !equalStrings(titles(first.Items), []string{"early row"}) {
		t.Fatalf("first snapshot: settled=%v rows=%v, want the early row while the provider runs", first.Settled, titles(first.Items))
	}
	clock.AwaitTimerDueIn(HardBudget - FirstSnapshot)
	clock.Advance(HardBudget - FirstSnapshot)
	settled := sink.untilSettled(t, gen)
	if got := titles(settled[len(settled)-1].Items); !equalStrings(got, []string{"early row"}) {
		t.Fatalf("settled rows after the hard budget = %v, want only the early row", got)
	}

	close(release)
	for {
		snap := sink.next(t)
		if snap.Gen != gen {
			continue
		}
		if containsTitle(snap.Items, "late row") {
			break
		}
	}
	if slowCount := statsOf(t, c, "slow").Slow; slowCount != 1 {
		t.Fatalf("slow count = %d, want 1", slowCount)
	}
}

// TestRootStreamShowsEachSourceAsItAnswers checks that a streamed root shows the first source's rows before the second
// source answers, and settles once both have.
func TestRootStreamShowsEachSourceAsItAnswers(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot})
	release := make(chan struct{})
	root := streamedRoot{first: Item{Kind: KindApplications, Title: "fast row", Score: 1}, second: Item{Kind: KindFiles, Title: "slow row", Score: 2}, release: release}
	c.Query("", []Provider{root})
	passFirstSnapshot(t, c, clock)

	if first := sink.next(t); first.Settled || !equalStrings(titles(first.Items), []string{"fast row"}) {
		t.Fatalf("first root snapshot: settled=%v rows=%v, want the fast row while the second source runs", first.Settled, titles(first.Items))
	}
	close(release)
	last := sink.untilSettled(t, c.Gen())
	if got := titles(last[len(last)-1].Items); !equalStrings(got, []string{"fast row", "slow row"}) {
		t.Fatalf("settled root rows = %v, want both sources", got)
	}
}

// TestSynchronousStreamedRootIsComplete checks that a synchronous query returns a settled result from a streamed root.
func TestSynchronousStreamedRootIsComplete(t *testing.T) {
	release := make(chan struct{})
	close(release)
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true})
	root := streamedRoot{first: Item{Kind: KindApplications, Title: "one", Score: 1}, second: Item{Kind: KindFiles, Title: "two", Score: 2}, release: release}
	snap, ready := c.Query("", []Provider{root})
	if !ready || !snap.Settled || len(snap.Items) != 2 {
		t.Fatalf("synchronous root: ready=%v settled=%v rows=%v, want two settled rows", ready, snap.Settled, titles(snap.Items))
	}
}

func containsTitle(items []Item, title string) bool {
	for _, it := range items {
		if it.Title == title {
			return true
		}
	}
	return false
}
