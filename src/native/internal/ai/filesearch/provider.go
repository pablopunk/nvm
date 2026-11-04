package filesearch

import (
	"context"
	"errors"
	"iter"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/search"
)

// DecisionBudget is how long one decision may take: the backend's decision route budget, as the Electron feature
// allows its AI call 30 seconds. The coordinator cuts the call off at this budget, not at the local one.
const DecisionBudget = 30 * time.Second

// Searcher is the part of the file index the provider reads the recent files from.
type Searcher interface {
	Search(ctx context.Context, q files.Query) iter.Seq[files.Batch]
}

// Options configures a Provider.
type Options struct {
	// Decider returns the decision client of the signed-in account, or nil when there is none. A nil decider answers
	// nothing, so a signed-out palette shows no AI rows.
	Decider func() Decider
	// Index is the file index whose most recent files are the candidates.
	Index Searcher
	// Row turns one matched file into a palette row.
	Row func(files.Result) search.Item
	// ScoreBase ranks the first match; the others rank one lower each, below local matches that score higher.
	ScoreBase int
}

// Provider is a search provider for the matches of a decision. It answers only typed queries: the decision is made
// for the text the user asked for, and never for the empty query.
type Provider struct {
	opts Options
}

// NewProvider returns the provider over opts.
func NewProvider(opts Options) Provider {
	return Provider{opts: opts}
}

// ID names the provider in statistics and in the coordinator's rows.
func (p Provider) ID() string { return "files-meaning" }

// Budget gives the decision call its own budget, so the coordinator does not cut it off at the local limit.
func (p Provider) Budget() search.Budget {
	return search.Budget{Soft: search.SoftBudget, Hard: DecisionBudget}
}

// Root answers the empty query with nothing.
func (p Provider) Root(context.Context) []search.Item { return nil }

// Search asks the decision model about the recent files for q and emits the matches. A failed decision emits one
// contextual row that says why, so the palette explains the failure in place.
func (p Provider) Search(ctx context.Context, q string, emit func(search.Batch)) {
	decider := p.decider()
	if decider == nil || q == "" {
		emit(search.Batch{Final: true})
		return
	}
	candidates := p.recent(ctx)
	names := make([]string, len(candidates))
	for i, candidate := range candidates {
		names[i] = candidate.Name
	}
	matches, err := Judge(ctx, decider, q, names)
	if ctx.Err() != nil {
		emit(search.Batch{Final: true})
		return
	}
	if err != nil {
		emit(search.Batch{Items: []search.Item{p.failureRow(err)}, Final: true})
		return
	}
	items := make([]search.Item, 0, len(matches))
	for rank, index := range matches {
		row := p.opts.Row(candidates[index])
		row.Kind = search.KindFiles
		row.Score = p.opts.ScoreBase - rank
		items = append(items, row)
	}
	emit(search.Batch{Items: items, Final: true})
}

// decider returns the client for this call, or nil when the account is signed out.
func (p Provider) decider() Decider {
	if p.opts.Decider == nil {
		return nil
	}
	return p.opts.Decider()
}

// recent returns the most recent files of the index, the candidates of the decision.
func (p Provider) recent(ctx context.Context) []files.Result {
	if p.opts.Index == nil {
		return nil
	}
	var last files.Batch
	for batch := range p.opts.Index.Search(ctx, files.Query{SortBy: files.SortRecent, Limit: CandidateLimit}) {
		last = batch
	}
	return last.Results
}

// failureRow is the palette row that says why the decision did not run. Backend messages are shown as the backend sent
// them; anything else gets a plain statement.
func (p Provider) failureRow(err error) search.Item {
	message := "Could not search files by meaning."
	var backendErr *backend.Error
	if errors.As(err, &backendErr) && backendErr.Message != "" {
		message = "Could not search files by meaning: " + backendErr.Message
	}
	return search.Item{ID: "files-meaning-error", Kind: search.KindFiles, Title: message, Contextual: true}
}
