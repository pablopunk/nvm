package search

import (
	"context"
	"strings"
)

// Query starts the search for q with the given providers and returns the rows ready at once: the rows
// remembered for q (ok), or, in Synchronous mode, the complete result. Without a remembered result the
// caller shows nothing new until the first snapshot arrives through OnSnapshot. In SingleSnapshot mode a typed
// query publishes only when every source has completed or been cut off. Its rows then stay, in their order: asking
// again for the query on screen, or an exact cache hit, revalidates the providers and only updates those rows' content.
// Query never waits for a provider. An empty q asks the providers for their root rows.
func (c *Coordinator) Query(q string, providers []Provider) (Snapshot, bool) {
	c.mu.Lock()
	fixed := c.opts.SingleSnapshot && q != ""
	revalidate := false
	if fixed && c.query == q && c.cancel != nil {
		if visible := c.single.visible; visible == nil || visible.Query != q {
			// The query's first result is still on its way; it reads the state as its providers answer.
			c.mu.Unlock()
			return Snapshot{}, false
		}
		revalidate = true
	}
	if c.opts.SingleSnapshot && !revalidate {
		c.single.begin(q)
		if previous := c.single.previous; previous != nil && len(previous.Items) > 0 {
			for _, provider := range providers {
				if provider.ID() == previous.Items[0].Provider {
					c.single.seed, _ = provider.(NarrowingSeedProvider)
					break
				}
			}
		}
	}
	previousQuery := c.query
	c.resetLocked()
	gen := c.gen
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.query = q
	c.queryCacheEpoch = c.cacheEpoch
	c.slots = make([]*slot, len(providers))
	for i, p := range providers {
		c.slots[i] = &slot{id: p.ID(), appended: isAppended(p), started: c.opts.Clock.Now(), budget: budgetOf(p), selfBudgeted: ownsCallBudgets(p), done: make(chan struct{}), refreshing: true}
	}
	c.pending = len(providers)
	c.emitted = false
	c.superseded = c.shown
	remembered, hit := c.cache.get(q)
	if revalidate {
		// The rows on screen anchor the refresh; the cache is not shown again.
		hit = false
	}
	c.warmSettled = hit && remembered.settled
	var warm Snapshot
	if !c.opts.SingleSnapshot && !hit && q != "" && previousQuery != "" && strings.HasPrefix(q, previousQuery) && q != previousQuery {
		for i, p := range providers {
			if seed, ok := p.(NarrowingSeedProvider); ok {
				c.slots[i].items = c.learned(seed.SeedNarrowing(q, c.shown[p.ID()]), q)
			}
		}
		rows := mergeCached(c.slots, c.limit(), true, c.opts.Flat, false)
		if len(rows) > 0 {
			warm = Snapshot{Gen: gen, Query: q, Items: rows, Pending: true}
			hit = true
			c.emitted = true
			c.shown = c.providerCache(rows)
		}
	}
	if hit && warm.Items == nil {
		for _, s := range c.slots {
			s.items = append([]Item(nil), remembered.providers[s.id]...)
		}
		limit := c.limit()
		if fixed {
			limit = 0
		}
		warm = Snapshot{Gen: gen, Query: q, Items: mergeCached(c.slots, limit, q != "", c.opts.Flat, remembered.settled), Settled: remembered.settled}
		if fixed {
			warm.Items = firstN(c.single.refine(q, warm.Items), c.limit())
			c.single.setVisible(warm)
		}
		if !c.opts.Synchronous {
			c.emitted = true
			c.shown = c.providerCache(warm.Items)
		}
	}
	if revalidate {
		warm = *c.single.visible
		warm.Gen = gen
		hit = true
		c.emitted = true
	}
	c.mu.Unlock()

	if c.opts.Synchronous {
		for i, p := range providers {
			c.invoke(ctx, gen, i, p, q)
		}
		snap := c.current(gen)
		if fixed {
			c.mu.Lock()
			c.single.setVisible(snap)
			c.mu.Unlock()
		}
		return snap, true
	}
	if len(providers) == 0 {
		snap := c.current(gen)
		if fixed {
			c.mu.Lock()
			c.single.setVisible(snap)
			c.mu.Unlock()
		}
		return snap, true
	}
	for i, p := range providers {
		c.start(ctx, gen, i, p, q)
	}
	if !fixed {
		go c.firstSnapshotTimer(gen)
	}
	if !hit {
		return Snapshot{}, false
	}
	return warm, true
}
