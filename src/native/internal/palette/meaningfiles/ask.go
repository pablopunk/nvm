package meaningfiles

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/search"
)

// askRowID names the row that asks the decision model. Its ID is stable, so the palette keeps the cursor on it.
const askRowID = "files-meaning-ask"

// askProvider offers the one row that starts the decision for the typed query. It is absent while the account is
// signed out, when there is no query, and once the query has been asked.
type askProvider struct {
	asked    *askedQuery
	signedIn func() bool
	// askRow builds the row for q; the view supplies the action that records the choice and refreshes.
	askRow func(q string) search.Item
}

// ID names the provider in statistics.
func (a askProvider) ID() string { return "files-meaning-ask" }

// Root answers the empty query with nothing.
func (a askProvider) Root(context.Context) []search.Item { return nil }

// Search emits the ask row for q, unless q is already asked or the account cannot ask.
func (a askProvider) Search(_ context.Context, q string, emit func(search.Batch)) {
	if q == "" || a.asked.is(q) || (a.signedIn != nil && !a.signedIn()) {
		emit(search.Batch{Final: true})
		return
	}
	emit(search.Batch{Items: []search.Item{a.askRow(q)}, Final: true})
}
