package search

import (
	"slices"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestSingleSnapshotColdQueryCompletesWhenProvidersNeverAnswer is a cold query whose local provider and aggregate
// never answer. The query still publishes once, within AggregateHardBudget, with the rows of the provider that did.
func TestSingleSnapshotColdQueryCompletesWhenProvidersNeverAnswer(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot, Flat: true, Tiered: true, SingleSnapshot: true})
	defer c.Cancel()
	aggregate := selfBudgetedSingle{singleProvider{id: "extensions", calls: make(chan chan singleAnswer, 1)}}
	healthy := fixedRows("healthy", Item{ID: "healthy", Title: "Quest", Score: 1})
	if _, ready := c.Query("q", []Provider{blockingProvider("stuck"), aggregate, healthy}); ready {
		t.Fatal("cold query was ready before its providers answered")
	}
	<-aggregate.calls
	clock.AwaitTimers(6) // soft and hard budget watches of the three providers
	awaitAnsweredExcept(t, c, "stuck", "extensions")
	clock.Advance(HardBudget)
	assertNoSnapshot(t, sink)
	clock.Advance(AggregateHardBudget - HardBudget)
	snapshots := sink.untilSettled(t, c.Gen())
	if len(snapshots) != 1 {
		t.Fatalf("cold query published %d snapshots, want one", len(snapshots))
	}
	if got := rowIDs(snapshots[0].Items); !slices.Equal(got, []string{"healthy"}) {
		t.Fatalf("rows = %v, want the answered provider's row", got)
	}
}

// hungAppAggregate is the root extension aggregate: self-budgeted, and able to vouch for a previously shown app row.
type hungAppAggregate struct{ selfBudgetedSingle }

func (hungAppAggregate) SeedNarrowing(q string, previous []Item) []Item {
	var out []Item
	for _, item := range previous {
		if item.Kind == KindApplications && MatchesName(q, item.Title) {
			out = append(out, item)
		}
	}
	return out
}

// TestNarrowingKeepsLeaderWhenItsProviderNeverAnswers types "lo", which shows Loom first, then "loo", whose app source
// never answers. Loom still matches, so it stays first, ahead of a command that matches "loo" exactly.
func TestNarrowingKeepsLeaderWhenItsProviderNeverAnswers(t *testing.T) {
	clock := fake.NewClock()
	sink := newSnapshotSink()
	c := NewCoordinator(Options{Clock: clock, OnSnapshot: sink.OnSnapshot, Flat: true, Tiered: true, SingleSnapshot: true})
	defer c.Cancel()
	apps := hungAppAggregate{selfBudgetedSingle{singleProvider{id: "extensions", calls: make(chan chan singleAnswer, 2)}}}
	commands := fixedRows("commands", Item{ID: "loo", Title: "Loo", Kind: KindCommands})
	c.Query("lo", []Provider{apps})
	clock.AwaitTimers(2) // the first query's budget watch is running before the second query starts
	answerSingle(<-apps.calls, []Item{{ID: "/Applications/Loom.app", Title: "Loom", Kind: KindApplications}}, true)
	first := sink.untilSettled(t, c.Gen())
	if got := rowIDs(first[len(first)-1].Items); len(got) == 0 || got[0] != "/Applications/Loom.app" {
		t.Fatalf("lo = %v, want Loom first", got)
	}
	c.Query("loo", []Provider{apps, commands})
	<-apps.calls
	clock.AwaitTimers(6) // the first query's two watches, then two for each provider of the second
	awaitAnsweredExcept(t, c, "extensions")
	clock.Advance(AggregateHardBudget)
	narrowed := sink.untilSettled(t, c.Gen())
	if got := rowIDs(narrowed[len(narrowed)-1].Items); !slices.Equal(got, []string{"/Applications/Loom.app", "loo"}) {
		t.Fatalf("loo = %v, want Loom kept first", got)
	}
	again, ready := c.Query("loo", []Provider{apps, commands})
	if !ready || !slices.Equal(rowIDs(again.Items), rowIDs(narrowed[len(narrowed)-1].Items)) {
		t.Fatalf("the same query changed: %v", rowIDs(again.Items))
	}
}

// awaitAnsweredExcept waits until every provider of the current query but the blocked ones has answered, so advancing
// the fake clock cuts off only the providers meant to miss their deadline.
func awaitAnsweredExcept(t *testing.T, c *Coordinator, blocked ...string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !answeredExcept(c, blocked) {
		if time.Now().After(deadline) {
			t.Fatal("providers never answered")
		}
		time.Sleep(time.Millisecond)
	}
}
