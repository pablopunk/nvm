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
	epoch   uint64
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

// reset keeps the last snapshot visible while the next root revalidates every extension.
func (c *rootRowCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filled, c.pending = false, nil
	c.epoch++
}

// stream stages a complete refresh before publishing it to subsequent queries. An interrupted pass leaves the
// previous catalog intact; observers never capture a mixture based on extension completion order.
func (c *rootRowCache) stream(ctx context.Context, m *manager.Manager, emit func(manager.SearchResult)) {
	scope, epoch := c.takeScope()
	answers := map[string]manager.SearchResult{}
	receive := func(result manager.SearchResult) { answers[result.ExtensionID] = result }
	if scope == nil {
		m.RootsEach(ctx, receive)
	} else {
		m.RootsOfEach(ctx, scope, receive)
	}
	for _, result := range c.commitPass(ctx, epoch, scope, answers) {
		emit(result)
	}
}

// commitPass publishes a whole refresh under one lock. Failed calls retain their prior rows; extensions absent from
// a completed pass lose theirs. A cancelled or reset pass cannot replace the catalog.
func (c *rootRowCache) commitPass(ctx context.Context, epoch uint64, scope map[string]bool, answers map[string]manager.SearchResult) []manager.SearchResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || epoch != c.epoch {
		return nil
	}
	if c.rows == nil {
		c.rows = map[string]manager.SearchResult{}
	}
	var changed []manager.SearchResult
	for id := range c.rows {
		if _, answered := answers[id]; !answered && (scope == nil || scope[id]) {
			delete(c.rows, id)
			changed = append(changed, manager.SearchResult{ExtensionID: id})
		}
	}
	for id, result := range answers {
		if result.Failure == nil {
			c.rows[id] = result
			changed = append(changed, result)
		}
	}
	if scope == nil {
		c.filled = true
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].ExtensionID < changed[j].ExtensionID })
	return changed
}

// takeScope returns the pending extensions and clears them, or nil when the root asks every extension.
func (c *rootRowCache) takeScope() (map[string]bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	scope := c.pending
	c.pending = nil
	if !c.filled || len(scope) == 0 {
		return nil, c.epoch
	}
	return scope, c.epoch
}

// remember stores the rows of one extension. An answer that comes after the pass was cancelled is a cancellation, not the
// extension's rows, so it is not stored.
func (c *rootRowCache) remember(ctx context.Context, epoch uint64, result manager.SearchResult) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || epoch != c.epoch || result.Failure != nil {
		return false
	}
	if c.rows == nil {
		c.rows = map[string]manager.SearchResult{}
	}
	c.rows[result.ExtensionID] = result
	return true
}

// settle ends a pass that was not cut off. The extensions the pass asked and did not hear from lose their rows, and a
// full pass marks the cache filled, so later refreshes ask only the pending extensions.
func (c *rootRowCache) settle(ctx context.Context, epoch uint64, scope map[string]bool, answered map[string]bool) []string {
	var removed []string
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || epoch != c.epoch {
		return nil
	}
	for id := range c.rows {
		if !answered[id] && (scope == nil || scope[id]) {
			delete(c.rows, id)
			removed = append(removed, id)
		}
	}
	if scope == nil {
		c.filled = true
	}
	return removed
}

// cachedOutside returns the cached rows of the extensions not in scope, in extension id order.
func (c *rootRowCache) cachedOutside(scope map[string]bool) []manager.SearchResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []manager.SearchResult
	for id, result := range c.rows {
		if !scope[id] {
			out = append(out, result)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExtensionID < out[j].ExtensionID })
	return out
}
