package extview

import (
	"context"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// searchProviders are the providers of a view with a Search handler: its answer to the typed text, then the second
// batch of its onSearchChange handler. They run off the UI thread, so the search text and the second batch are read
// as the palette asked for them.
func (s *Session) searchProviders() []search.Provider {
	return []search.Provider{
		viewSearchProvider{session: s, ref: s.searchRef()},
		secondBatchProvider{session: s, query: strings.TrimSpace(s.searchChange.query), items: s.searchChange.items, pending: s.searchChange.inFlight},
	}
}

// answerSearch runs the handler for text under ctx. A cancelled or failed call has no rows.
func (s *Session) answerSearch(ctx context.Context, ref api.ActionRef, text string) ([]api.Item, bool) {
	if s.deps.Answer == nil {
		return nil, false
	}
	result, failure := s.deps.Answer(ctx, s.extID, api.Invocation{Ref: ref, Value: text})
	if ctx.Err() != nil || failure != nil || result.Patch == nil {
		return nil, false
	}
	return result.Patch.Items, true
}

// searchRef is the handler's reference with the search accessory's value under its id, so a grid with categories
// answers each query for the category shown.
func (s *Session) searchRef() api.ActionRef {
	return s.withAccessoryArg(*s.view.Search)
}

// withAccessoryArg is ref with the search accessory's value under its id in Args, when the view has an accessory.
func (s *Session) withAccessoryArg(ref api.ActionRef) api.ActionRef {
	if accessory := s.view.Filter; accessory != nil && s.accessory != nil && accessory.ID != "" {
		args := make(map[string]string, len(ref.Args)+1)
		for key, value := range ref.Args {
			args[key] = value
		}
		args[accessory.ID] = s.accessory.Value
		ref.Args = args
	}
	return ref
}

// viewSearchProvider answers a query with the rows its view's search handler returns, in the order it returns them.
type viewSearchProvider struct {
	session *Session
	ref     api.ActionRef
}

func (p viewSearchProvider) ID() string { return "view-search" }

func (p viewSearchProvider) Root(ctx context.Context) []search.Item { return p.rows(ctx, "") }

// Search asks the handler for q and emits its rows. The second batch is asked for after the text rests, on the UI thread.
func (p viewSearchProvider) Search(ctx context.Context, q string, emit func(search.Batch)) {
	text := strings.TrimSpace(q)
	p.session.onUI(func() { p.session.scheduleSearchChange(text) })
	emit(search.Batch{Items: p.rows(ctx, text), Final: true})
}

// rows calls the handler and waits for its answer. A cancelled call, or a failed one, has no rows.
func (p viewSearchProvider) rows(ctx context.Context, text string) []search.Item {
	items, ok := p.session.answerSearch(ctx, p.ref, text)
	if !ok {
		return nil
	}
	out := make([]search.Item, 0, len(items))
	for i, item := range items {
		if !itemSupported(p.session.deps, item) {
			continue
		}
		row := p.session.row(item)
		row.Score = len(items) - i
		out = append(out, row)
	}
	return out
}

// secondBatchProvider shows the rows the onSearchChange handler answered for the text, and says while the handler runs.
type secondBatchProvider struct {
	session *Session
	query   string
	items   []api.Item
	pending bool
}

func (p secondBatchProvider) ID() string { return "view-second-batch" }

func (p secondBatchProvider) Root(context.Context) []search.Item { return nil }

// Search emits the batch when it answers the text, and emits it as pending while the handler is still running for it.
func (p secondBatchProvider) Search(_ context.Context, q string, emit func(search.Batch)) {
	text := strings.TrimSpace(q)
	if p.pending && p.query == text {
		emit(search.Batch{Pending: true})
		return
	}
	emit(search.Batch{Items: p.rowsFor(text), Final: true})
}

// rowsFor are the rows of the batch for text, or none when the batch answers another text.
func (p secondBatchProvider) rowsFor(text string) []search.Item {
	if p.query != text || len(p.items) == 0 {
		return nil
	}
	out := make([]search.Item, 0, len(p.items))
	for _, item := range p.items {
		if !itemSupported(p.session.deps, item) {
			continue
		}
		out = append(out, p.session.row(item))
	}
	return out
}
