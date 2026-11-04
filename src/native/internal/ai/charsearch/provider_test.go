package charsearch

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// watchdog bounds how long a test waits for a goroutine. It is a guard, never a timing assumption.
const watchdog = 5 * time.Second

// rowsOf turns the matches into rows, one per character, as the picker's grid does.
func rowsOf(matches []Record) []search.Item {
	items := make([]search.Item, 0, len(matches))
	for _, r := range matches {
		items = append(items, search.Item{ID: r.Key, Kind: "Emoji", Title: r.Name, Glyph: r.Glyph})
	}
	return items
}

// providerFor returns the provider over the test catalog, with the typing pause of the clock.
func providerFor(model Decider, clock *fake.Clock, pause time.Duration) Provider {
	return NewProvider(Options{Decider: model, Matcher: NewMatcher(testCatalog, fake.NewClock()), Category: allCategory, Clock: clock, Pause: pause, Rows: rowsOf})
}

// collect runs a search and returns the batches it emitted.
func collect(p Provider, ctx context.Context, q string) []search.Batch {
	var batches []search.Batch
	p.Search(ctx, q, func(b search.Batch) { batches = append(batches, b) })
	return batches
}

func TestTheSecondBatchIsAppendedAndHasItsOwnBudget(t *testing.T) {
	p := providerFor(cuteModel(), fake.NewClock(), 0)
	if appended, ok := any(p).(search.AppendedProvider); !ok || !appended.Appended() {
		t.Fatal("the second batch must follow the ranked rows")
	}
	if got := p.Budget().Hard; got != DecisionBudget {
		t.Fatalf("hard budget = %v, want the decision budget", got)
	}
}

func TestAWithoutADeciderEmitsNothing(t *testing.T) {
	batches := collect(providerFor(nil, fake.NewClock(), 0), context.Background(), "kitten time")
	if len(batches) != 1 || !batches[0].Final || len(batches[0].Items) != 0 {
		t.Fatalf("batches = %+v, want one final empty batch", batches)
	}
}

func TestTheMatchesBecomeRowsOfOneFinalBatch(t *testing.T) {
	batches := collect(providerFor(cuteModel(), fake.NewClock(), 0), context.Background(), "Cute ")
	if len(batches) != 1 || !batches[0].Final || len(batches[0].Items) != 3 || batches[0].Items[0].ID != "cat" {
		t.Fatalf("batches = %+v, want the three matches in one final batch", batches)
	}
}

func TestASearchWaitsForTheTypingPauseBeforeAsking(t *testing.T) {
	clock := fake.NewClock()
	model := cuteModel()
	emitted := make(chan search.Batch, 1)
	go providerFor(model, clock, TypingPause).Search(context.Background(), "Cute ", func(b search.Batch) { emitted <- b })
	clock.AwaitTimers(1)
	if len(model.calls) != 0 {
		t.Fatal("the model was asked before the text rested")
	}
	clock.Advance(TypingPause)
	select {
	case batch := <-emitted:
		if len(batch.Items) != 3 {
			t.Fatalf("items = %d, want the three matches", len(batch.Items))
		}
	case <-time.After(watchdog):
		t.Fatal("the search never emitted")
	}
}

func TestACancelledSearchNeverAsks(t *testing.T) {
	clock := fake.NewClock()
	model := cuteModel()
	ctx, cancel := context.WithCancel(context.Background())
	emitted := make(chan search.Batch, 1)
	go providerFor(model, clock, TypingPause).Search(ctx, "Cute ", func(b search.Batch) { emitted <- b })
	clock.AwaitTimers(1)
	cancel()
	select {
	case batch := <-emitted:
		if len(batch.Items) != 0 || !batch.Final {
			t.Fatalf("batch = %+v, want nothing from a cancelled search", batch)
		}
	case <-time.After(watchdog):
		t.Fatal("the cancelled search never emitted")
	}
	if len(model.calls) != 0 {
		t.Fatal("a cancelled search asked the model")
	}
}

func TestAFailedDecisionEmitsNothingAndTheLocalRowsStand(t *testing.T) {
	model := &fakeModel{failure: errUnavailable}
	batches := collect(providerFor(model, fake.NewClock(), 0), context.Background(), "kitten time")
	if len(batches) != 1 || len(batches[0].Items) != 0 || !batches[0].Final {
		t.Fatalf("batches = %+v, want one final empty batch", batches)
	}
}
