package search

// A flat result changes once per keystroke. The first snapshot of a query waits until every ranked provider has
// answered or FirstSnapshot has passed; after that, rows already on screen keep their order. Rows that arrive later
// follow them, above the query fallbacks, and a row leaves only when its provider's newer answer no longer has it. The remembered result of a
// query keeps the full ranking, so the next visit shows the rows in their best order at once.

// readyLocked reports whether the current generation may publish: a snapshot is already on screen, or every ranked
// provider has finished. Appended providers never hold up the first snapshot. The caller holds c.mu.
func (c *Coordinator) readyLocked() bool {
	if c.emitted || c.opts.Synchronous {
		return true
	}
	for _, s := range c.slots {
		if !s.appended && !s.finished {
			return false
		}
	}
	return true
}

// stableLocked returns fresh in the order the screen already shows: the committed rows that are still part of the
// answer, then the new rows in ranked order. A committed row whose provider has not answered again yet stays. Only a
// flat result is kept stable: a sectioned one is grouped by kind, so its order is the sections'. The caller holds c.mu.
func (c *Coordinator) stableLocked(fresh []Item) []Item {
	if c.opts.Synchronous || !c.opts.Flat || c.committed == nil {
		return fresh
	}
	byKey := make(map[cachedItemKey]Item, len(fresh))
	for _, item := range fresh {
		byKey[cacheKey(item)] = item
	}
	waiting := c.unansweredLocked()
	out := make([]Item, 0, len(fresh))
	placed := make(map[cachedItemKey]bool, len(fresh))
	for _, item := range c.committed {
		key := cacheKey(item)
		if current, ok := byKey[key]; ok {
			out, placed[key] = append(out, current), true
		} else if waiting[c.committedFrom[key]] {
			out, placed[key] = append(out, item), true
		}
	}
	for _, item := range fresh {
		if key := cacheKey(item); !placed[key] {
			out, placed[key] = append(out, item), true
		}
	}
	return firstN(fallbacksLast(out), c.limit())
}

// fallbacksLast moves the query fallbacks (web search, Automate) below every other row, keeping each group's order:
// a late answer never lands under the rows that only echo the query.
func fallbacksLast(items []Item) []Item {
	out := make([]Item, 0, len(items))
	var fallbacks []Item
	for _, item := range items {
		if item.QueryFallback {
			fallbacks = append(fallbacks, item)
		} else {
			out = append(out, item)
		}
	}
	return append(out, fallbacks...)
}

// unansweredLocked lists the providers that have not answered this generation yet. The caller holds c.mu.
func (c *Coordinator) unansweredLocked() map[string]bool {
	waiting := map[string]bool{}
	for _, s := range c.slots {
		if s.refreshing && !s.finished {
			waiting[s.id] = true
		}
	}
	return waiting
}

// commitLocked records the rows now on screen and the provider each came from. The caller holds c.mu.
func (c *Coordinator) commitLocked(items []Item) {
	c.committed = items
	c.committedFrom = make(map[cachedItemKey]string, len(items))
	for _, s := range c.slots {
		for _, item := range s.items {
			c.committedFrom[cacheKey(item)] = s.id
		}
	}
}
