package search

import "context"

// Provider is one source of rows for the root of the palette. Root answers the empty query; Search answers
// a typed one and may call emit several times with partial rows, the last with Final set.
//
// Providers run off the UI thread, so they must not read state the UI thread changes: the caller captures
// what a query needs when it starts, and the provider works on that capture. A provider should honor ctx:
// the coordinator cancels it when the query changes or the call runs past the hard budget.
type Provider interface {
	// ID names the provider in statistics and logs. It must be unique among the providers of one search.
	ID() string
	// Root returns the rows for the empty query.
	Root(ctx context.Context) []Item
	// Search emits the rows for q, which is never empty.
	Search(ctx context.Context, q string, emit func(Batch))
}

// Batch is the rows one provider found for one query. Provider is filled in by the coordinator, so a
// provider cannot answer for another one.
type Batch struct {
	Provider string
	Items    []Item
	Final    bool
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
