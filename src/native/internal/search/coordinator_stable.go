package search

// readyLocked waits for every source in single-snapshot mode; streaming views may publish before appended sources.
func (c *Coordinator) readyLocked() bool {
	if c.opts.SingleSnapshot {
		return c.pending == 0
	}
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
