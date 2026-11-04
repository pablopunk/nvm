package app

import (
	"context"
	"sort"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

// rootRowCache keeps each extension's last empty-query rows. An extension that asks for a root refresh (InvalidateRoot)
// marks itself pending, and the next root asks only the pending extensions again; the others keep their rows. 1.x
// clears only the caller's root cache entry the same way. Until the cache is filled, and after a state change resets it,
// the root asks every extension.
type rootRowCache struct {
	mu      sync.Mutex
	filled  bool
	pending map[string]bool
	rows    map[string]manager.SearchResult
}

// markPending records that extension id asked for its root rows to run again.
func (c *rootRowCache) markPending(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending == nil {
		c.pending = map[string]bool{}
	}
	c.pending[id] = true
}

// reset forgets every row and pending refresh, so the next root asks every extension.
func (c *rootRowCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filled, c.pending, c.rows = false, nil, nil
}

// results returns the root rows of every extension, asking only the pending ones when a refresh is pending.
func (c *rootRowCache) results(ctx context.Context, m *manager.Manager) []manager.SearchResult {
	scope := c.takeScope()
	if scope == nil {
		fresh := m.Roots(ctx)
		c.storeAll(fresh)
		return fresh
	}
	c.storeScoped(m.RootsOf(ctx, scope), scope)
	return c.sorted()
}

// takeScope returns the pending extensions and clears them, or nil when the root asks every extension.
func (c *rootRowCache) takeScope() map[string]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	scope := c.pending
	c.pending = nil
	if !c.filled || len(scope) == 0 {
		return nil
	}
	return scope
}

func (c *rootRowCache) storeAll(fresh []manager.SearchResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filled = true
	c.rows = map[string]manager.SearchResult{}
	for _, result := range fresh {
		c.rows[result.ExtensionID] = result
	}
}

// storeScoped replaces the rows of the extensions in scope with fresh. An extension in scope that answered nothing
// (not running, or no longer enabled) loses its rows.
func (c *rootRowCache) storeScoped(fresh []manager.SearchResult, scope map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rows == nil {
		c.rows = map[string]manager.SearchResult{}
	}
	for id := range scope {
		delete(c.rows, id)
	}
	for _, result := range fresh {
		c.rows[result.ExtensionID] = result
	}
}

// sorted returns the cached rows in extension id order, as the manager returns them.
func (c *rootRowCache) sorted() []manager.SearchResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]manager.SearchResult, 0, len(c.rows))
	for _, result := range c.rows {
		out = append(out, result)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExtensionID < out[j].ExtensionID })
	return out
}
