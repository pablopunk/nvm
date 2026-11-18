package search

// readyLocked reports whether the current generation may publish. Appended providers never hold it up.
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

// stableLocked uses the merge's rank order, independent of arrival order. Slots retain their own cached
// rows until an authoritative batch replaces them.
func (c *Coordinator) stableLocked(fresh []Item) []Item { return fresh }

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
