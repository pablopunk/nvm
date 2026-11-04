package palette

import (
	"strings"
	"sync"
	"time"
)

// searchRefreshDelay is how long a refresh request waits for others, so a burst of answers from a background source
// costs one search.
const searchRefreshDelay = 30 * time.Millisecond

// searchRefresher collects the queries background sources asked the palette to search again. It is shared by the
// goroutines that ask and the UI thread that answers.
type searchRefresher struct {
	mu      sync.Mutex
	queries map[string]bool
	armed   bool
}

// request records query and reports whether the caller must schedule the search: no search is scheduled yet.
func (r *searchRefresher) request(query string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.queries == nil {
		r.queries = map[string]bool{}
	}
	r.queries[query] = true
	if r.armed {
		return false
	}
	r.armed = true
	return true
}

// take returns the recorded queries and clears them, so the next request schedules a search again.
func (r *searchRefresher) take() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	queries := r.queries
	r.queries, r.armed = nil, false
	return queries
}

// RefreshSearch asks the palette to search again for query, when it still shows that query and is open. An empty
// query is the root. It is safe from any goroutine and never blocks: the search runs after a short delay, on the UI
// thread, and requests that arrive within the delay share it.
func (p *Palette) RefreshSearch(query string) {
	if p.win == nil || !p.refresh.request(strings.TrimSpace(query)) {
		return
	}
	go func() {
		<-p.opts.Clock.After(searchRefreshDelay)
		p.win.Update(p.runSearchRefresh)
	}()
}

// runSearchRefresh searches again when the palette shows one of the queries that were asked for. The cursor stays
// where it is, as for Refresh.
func (p *Palette) runSearchRefresh() {
	queries := p.refresh.take()
	if !p.win.IsVisible() || !queries[strings.TrimSpace(p.query)] {
		return
	}
	keep := p.chosen
	p.invalidateRows()
	p.search()
	p.choose(keep)
}
