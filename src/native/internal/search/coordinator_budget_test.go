package search

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// blockingProvider answers only when its context ends, the way a provider stuck on a slow call behaves.
func blockingProvider(id string) Provider {
	return NewRowsProvider(id, func(ctx context.Context, _ string) []Item {
		<-ctx.Done()
		return nil
	})
}

// TestBlockedProviderDoesNotDelayTheFirstSnapshot checks that a provider that never answers within the budget
// neither holds back the rows of the others nor the snapshot that carries them. The clock is not advanced until
// the first snapshot has arrived.
func TestBlockedProviderDoesNotDelayTheFirstSnapshot(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot})
	fast := fixedRows("fast", Item{Kind: KindFiles, Title: "fast row", Score: 1})
	c.Query("q", []Provider{blockingProvider("blocked"), fast})

	first := sink.next(t)
	if first.Settled || !equalStrings(titles(first.Items), []string{"fast row"}) {
		t.Fatalf("first snapshot: settled=%v rows=%v, want the fast row while the other provider is pending", first.Settled, titles(first.Items))
	}

	clock.AwaitTimers(5)
	clock.Advance(HardBudget)
	last := sink.untilSettled(t, c.Gen())
	if got := titles(last[len(last)-1].Items); !equalStrings(got, []string{"fast row"}) {
		t.Fatalf("settled rows after the hard budget = %v", got)
	}
	slow := statsOf(t, c, "blocked")
	if slow.Slow != 1 {
		t.Fatalf("blocked provider slow count = %d, want 1", slow.Slow)
	}
}

// TestPanickingProviderStillSettlesTheQuery checks that a provider that panics is contained: its rows are
// dropped, the others are shown, and the query still settles.
func TestPanickingProviderStillSettlesTheQuery(t *testing.T) {
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: fake.NewClock(), OnSnapshot: sink.OnSnapshot})
	boom := NewRowsProvider("boom", func(context.Context, string) []Item { panic("provider bug") })
	fine := fixedRows("fine", Item{Kind: KindFiles, Title: "fine row", Score: 1})
	c.Query("q", []Provider{boom, fine})
	snaps := sink.untilSettled(t, c.Gen())
	if got := titles(snaps[len(snaps)-1].Items); !equalStrings(got, []string{"fine row"}) {
		t.Fatalf("settled rows = %v, want the other provider's row", got)
	}
}

// statsOf returns the counters of one provider, failing the test when it has never been called.
func statsOf(t *testing.T, c *Coordinator, id string) ProviderStats {
	t.Helper()
	for _, s := range c.Stats() {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("no statistics for provider %q", id)
	return ProviderStats{}
}

// budgetedBlocking is a provider that answers only when its context ends and declares its own budget.
type budgetedBlocking struct {
	id     string
	budget Budget
}

func (p budgetedBlocking) ID() string { return p.id }

func (p budgetedBlocking) Budget() Budget { return p.budget }

func (p budgetedBlocking) Root(ctx context.Context) []Item {
	<-ctx.Done()
	return nil
}

func (p budgetedBlocking) Search(ctx context.Context, _ string, _ func(Batch)) {
	<-ctx.Done()
}

// TestDeclaredBudgetReplacesTheLocalHardBudget checks that a provider that declares a longer budget is not cut off at
// the local hard budget, and is cut off at its own.
func TestDeclaredBudgetReplacesTheLocalHardBudget(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot})
	remote := budgetedBlocking{id: "remote", budget: Budget{Soft: time.Second, Hard: 30 * time.Second}}
	c.Query("q", []Provider{remote})

	clock.AwaitTimers(3)
	clock.Advance(HardBudget)
	if slow := statsOf(t, c, "remote").Slow; slow != 0 {
		t.Fatalf("remote provider cut off at the local budget: slow = %d", slow)
	}
	clock.Advance(remote.budget.Hard - HardBudget)
	sink.untilSettled(t, c.Gen())
	if slow := statsOf(t, c, "remote").Slow; slow != 1 {
		t.Fatalf("remote provider slow count = %d, want 1 after its own budget", slow)
	}
}

// TestProvidersWithoutADeclaredBudgetUseTheDefault checks the default budget is the local one.
func TestProvidersWithoutADeclaredBudgetUseTheDefault(t *testing.T) {
	if got := budgetOf(blockingProvider("local")); got != DefaultBudget {
		t.Fatalf("budget = %+v, want the default %+v", got, DefaultBudget)
	}
	if DefaultBudget.Hard != HardBudget || DefaultBudget.Soft != SoftBudget {
		t.Fatalf("default budget %+v does not match the PLAN.md limits", DefaultBudget)
	}
}
