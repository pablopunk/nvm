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
