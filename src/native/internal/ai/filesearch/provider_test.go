package filesearch

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeIndex answers the recent files and records the query it was asked.
type fakeIndex struct {
	results []files.Result
	queries []files.Query
}

func (f *fakeIndex) Search(_ context.Context, q files.Query) iter.Seq[files.Batch] {
	f.queries = append(f.queries, q)
	return func(yield func(files.Batch) bool) {
		yield(files.Batch{Results: f.results, Final: true})
	}
}

// collect runs a provider's search and returns every batch it emitted.
func collect(p Provider, ctx context.Context, q string) []search.Batch {
	var out []search.Batch
	p.Search(ctx, q, func(b search.Batch) { out = append(out, b) })
	return out
}

func rowFor(r files.Result) search.Item {
	return search.Item{ID: r.Path, Title: r.Name}
}

func recentFiles() []files.Result {
	return []files.Result{
		{Path: "/home/a/beach.jpg", Name: "beach.jpg"},
		{Path: "/home/a/tax.pdf", Name: "tax.pdf"},
		{Path: "/home/a/sunset.png", Name: "sunset.png"},
	}
}

func TestProviderEmitsTheMatchesAsRankedFileRows(t *testing.T) {
	decider := &recordingDecider{result: backend.DecisionResult{Answers: map[string]backend.DecisionAnswer{
		"file_0": predicate(0.9),
		"file_2": predicate(0.8),
	}}}
	index := &fakeIndex{results: recentFiles()}
	p := NewProvider(Options{
		Decider:   func() Decider { return decider },
		Index:     index,
		Row:       rowFor,
		ScoreBase: 100,
	})
	batches := collect(p, context.Background(), "holiday")
	if len(batches) != 1 || !batches[0].Final {
		t.Fatalf("batches = %+v, want one final batch", batches)
	}
	rows := batches[0].Items
	if len(rows) != 2 || rows[0].Title != "beach.jpg" || rows[1].Title != "sunset.png" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Score != 100 || rows[1].Score != 99 || rows[0].Kind != search.KindFiles {
		t.Fatalf("scores and kind = %d %d %s", rows[0].Score, rows[1].Score, rows[0].Kind)
	}
	if got := index.queries[0]; got.SortBy != files.SortRecent || got.Limit != CandidateLimit {
		t.Fatalf("index query = %+v, want the %d most recent files", got, CandidateLimit)
	}
}

func TestSignedOutProviderAsksNothing(t *testing.T) {
	index := &fakeIndex{results: recentFiles()}
	p := NewProvider(Options{Decider: func() Decider { return nil }, Index: index, Row: rowFor})
	batches := collect(p, context.Background(), "holiday")
	if len(batches) != 1 || len(batches[0].Items) != 0 || !batches[0].Final {
		t.Fatalf("batches = %+v, want one empty final batch", batches)
	}
	if len(index.queries) != 0 {
		t.Fatal("the index was read while signed out")
	}
}

func TestEmptyQueryAndRootAskNothing(t *testing.T) {
	decider := &recordingDecider{}
	p := NewProvider(Options{Decider: func() Decider { return decider }, Index: &fakeIndex{}, Row: rowFor})
	if rows := p.Root(context.Background()); rows != nil {
		t.Fatalf("root rows = %v, want none", rows)
	}
	collect(p, context.Background(), "")
	if len(decider.calls) != 0 {
		t.Fatal("a decision was made for the empty query")
	}
}

func TestFailedDecisionExplainsItselfInTheRows(t *testing.T) {
	decider := &recordingDecider{err: &backend.Error{Kind: backend.KindRateLimited, Message: "slow down"}}
	p := NewProvider(Options{Decider: func() Decider { return decider }, Index: &fakeIndex{results: recentFiles()}, Row: rowFor})
	rows := collect(p, context.Background(), "holiday")[0].Items
	if len(rows) != 1 || !rows[0].Contextual || rows[0].Title != "Could not search files by meaning: slow down" {
		t.Fatalf("rows = %+v, want one contextual failure row", rows)
	}
}

func TestPlainFailureGetsAPlainRow(t *testing.T) {
	decider := &recordingDecider{err: errors.New("socket closed")}
	p := NewProvider(Options{Decider: func() Decider { return decider }, Index: &fakeIndex{results: recentFiles()}, Row: rowFor})
	rows := collect(p, context.Background(), "holiday")[0].Items
	if len(rows) != 1 || rows[0].Title != "Could not search files by meaning." {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestCancelledSearchEmitsNoRows(t *testing.T) {
	decider := &recordingDecider{result: backend.DecisionResult{Answers: map[string]backend.DecisionAnswer{"file_0": predicate(1)}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewProvider(Options{Decider: func() Decider { return decider }, Index: &fakeIndex{results: recentFiles()}, Row: rowFor})
	batches := collect(p, ctx, "holiday")
	if len(batches) != 1 || len(batches[0].Items) != 0 {
		t.Fatalf("batches = %+v, want an empty final batch", batches)
	}
}

func TestProviderDeclaresTheDecisionBudget(t *testing.T) {
	p := NewProvider(Options{})
	budget := p.Budget()
	if budget.Hard != 30*time.Second || budget.Soft != search.SoftBudget {
		t.Fatalf("budget = %+v, want 30 s hard and the local soft budget", budget)
	}
	if p.ID() != "files-meaning" {
		t.Fatalf("id = %q", p.ID())
	}
}
