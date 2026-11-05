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

// reset forgets every row and pending refresh, so the next root asks every extension.
func (c *rootRowCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filled, c.pending = false, nil
	c.epoch++
}

// stream hands the root rows of the extensions to emit as each one answers, so a slow extension never holds back the
// others. A refresh that asks only the pending extensions emits the cached rows of the others at once. The pass is
// complete only when its context was not cancelled; then the extensions it did not hear from lose their rows.
func (c *rootRowCache) stream(ctx context.Context, m *manager.Manager, emit func(manager.SearchResult)) {
	c.mu.Lock()
	epoch := c.epoch
	c.mu.Unlock()
	scope := c.takeScope()
	for _, result := range c.cachedOutside(nil) {
		emit(result)
	}
	answered := map[string]bool{}
	receive := func(result manager.SearchResult) {
		answered[result.ExtensionID] = true
		if c.remember(ctx, epoch, result) {
			emit(result)
		}
	}
	if scope == nil {
		m.RootsEach(ctx, receive)
	} else {
		m.RootsOfEach(ctx, scope, receive)
	}
	c.mu.Lock()
	current := ctx.Err() == nil && epoch == c.epoch
	c.mu.Unlock()
	if current {
		for _, id := range c.settle(scope, answered) {
			emit(manager.SearchResult{ExtensionID: id})
		}
	}
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
func (c *rootRowCache) settle(scope map[string]bool, answered map[string]bool) []string {
	var removed []string
	c.mu.Lock()
	defer c.mu.Unlock()
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
