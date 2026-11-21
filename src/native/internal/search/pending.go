package search

// pendingCallsLocked reports whether a provider that has not finished has said its call is in flight. The caller
// holds c.mu.
func (c *Coordinator) pendingCallsLocked() bool {
	for _, s := range c.slots {
		if s.pending && !s.finished {
			return true
		}
	}
	return false
}
