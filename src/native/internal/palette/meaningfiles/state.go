package meaningfiles

import (
	"context"
	"sync"

	"github.com/pablopunk/nvm/native/internal/search"
)

// askedQuery remembers the query the user chose to send to the decision model. Providers read it on the coordinator's
// goroutines and the row writes it on the UI thread, so it is guarded.
type askedQuery struct {
	mu sync.Mutex
	q  string
}

// set records q as asked.
func (a *askedQuery) set(q string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.q = q
}

// is reports whether q is the asked query.
func (a *askedQuery) is(q string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return q != "" && a.q == q
}

// gatedProvider answers with the decision provider only for the query the user asked, so no decision runs on a
// keystroke. It keeps the decision provider's ID and budget.
type gatedProvider struct {
	inner search.Provider
	asked *askedQuery
}

// ID is the decision provider's ID.
func (g gatedProvider) ID() string { return g.inner.ID() }

// Budget is the decision provider's budget, when it declares one.
func (g gatedProvider) Budget() search.Budget {
	if budgeted, ok := g.inner.(search.BudgetedProvider); ok {
		return budgeted.Budget()
	}
	return search.DefaultBudget
}

// Root answers the empty query with nothing.
func (g gatedProvider) Root(context.Context) []search.Item { return nil }

// Search runs the decision provider when q is the asked query, and emits nothing otherwise.
func (g gatedProvider) Search(ctx context.Context, q string, emit func(search.Batch)) {
	if !g.asked.is(q) {
		emit(search.Batch{Final: true})
		return
	}
	g.inner.Search(ctx, q, emit)
}
