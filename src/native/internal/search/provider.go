package search

import (
	"context"
	"time"
)

// Provider is one source of rows for the root of the palette. Root answers the empty query; Search answers
// a typed one and may call emit several times with partial rows, the last with Final set.
//
// Providers run off the UI thread, so they must not read state the UI thread changes: the caller captures
// what a query needs when it starts, and the provider works on that capture. A provider should honor ctx:
// the coordinator cancels it when the query changes.
type Provider interface {
	// ID names the provider in statistics and logs. It must be unique among the providers of one search.
	ID() string
	// Root returns the rows for the empty query.
	Root(ctx context.Context) []Item
	// Search emits the rows for q, which is never empty.
	Search(ctx context.Context, q string, emit func(Batch))
}

// RootStreamer is a Provider whose empty-query rows come in batches, as its search rows do. A provider that gathers the
// root from several independent sources implements it, so the sources that answer first show before the others. Its
// last batch carries Final, as a search's does.
type RootStreamer interface {
	Provider
	RootStream(ctx context.Context, emit func(Batch))
}

// Budget is the time one call of a provider may take: Soft is when a call is logged as over budget and Hard is when
// the call is marked slow and stops holding up the snapshot. Its rows that arrive later still merge into the query.
// SelfBudgetedProvider aggregates calls that each enforce their own execution deadline. The coordinator
// stops waiting at Hard but accepts their late batches until they finish or the query changes.
type SelfBudgetedProvider interface {
	Provider
	OwnsCallBudgets()
}

type Budget struct {
	Soft, Hard time.Duration
}

// DefaultBudget is the budget of a provider that does not declare its own: the PLAN.md B8 limits for local work.
var DefaultBudget = Budget{Soft: SoftBudget, Hard: HardBudget}

// BudgetedProvider is a Provider whose calls need a budget other than the default. A source that asks a network
// service cannot answer inside the local limit, so it declares the longer budget its service needs; the coordinator
// then enforces that budget instead.
type BudgetedProvider interface {
	Provider
	Budget() Budget
}

// budgetOf returns the budget the coordinator enforces for p.
func budgetOf(p Provider) Budget {
	if budgeted, ok := p.(BudgetedProvider); ok {
		return budgeted.Budget()
	}
	return DefaultBudget
}

// Batch is the rows one provider found for one query. Provider is filled in by the coordinator, so a
// provider cannot answer for another one.
type Batch struct {
	Provider string
	Items    []Item
	// Replace replaces this provider's previous rows, for snapshot refreshes.
	Replace bool
	Final   bool
	// Pending says the provider has started a call that outlasts the moment the query was typed, and its rows are
	// still to come. It carries no rows, and Snapshot.Pending holds until the provider's Final batch.
	Pending bool
}

// RowsFunc computes the rows of one source for a query. An empty query asks for the root rows.
type RowsFunc func(ctx context.Context, q string) []Item

// rowsProvider is a Provider that answers each call with one batch of rows.
type rowsProvider struct {
	id   string
	rows RowsFunc
}

// NewRowsProvider returns a provider whose rows are computed in one piece per call: the common case of a
// source that ranks everything it has and returns its best rows. Providers that stream results implement
// Provider directly.
func NewRowsProvider(id string, rows RowsFunc) Provider {
	return rowsProvider{id: id, rows: rows}
}

func (p rowsProvider) ID() string { return p.id }

func (p rowsProvider) Root(ctx context.Context) []Item { return p.rows(ctx, "") }

func (p rowsProvider) Search(ctx context.Context, q string, emit func(Batch)) {
	emit(Batch{Items: p.rows(ctx, q), Final: true})
}
