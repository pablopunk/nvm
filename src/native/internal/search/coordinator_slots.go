package search

import "github.com/pablopunk/nvm/native/internal/logx"

// watch logs a call that runs past the soft budget and cuts it off at the hard one. Both timers start with the
// call, so the budgets hold however the scheduler orders them.
func (c *Coordinator) watch(gen, idx int) {
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	done, budget := s.done, s.budget
	c.mu.Unlock()
	soft := c.opts.Clock.After(budget.Soft)
	hard := c.opts.Clock.After(budget.Hard)
	select {
	case <-done:
		return
	case <-soft:
		logx.Default().Debug("search provider over its soft budget", logx.F{"provider": s.id, "soft_ms": budget.Soft.Milliseconds()})
	}
	select {
	case <-done:
	case <-hard:
		c.cutOff(gen, idx)
	}
}

// deliver merges a batch into its provider's slot and, for an asynchronous query, publishes the new snapshot.
func (c *Coordinator) deliver(gen, idx int, b Batch) {
	c.mu.Lock()
	if !c.acceptsLocked(gen, idx) {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	if s.refreshing && !(b.Pending && len(b.Items) == 0 && !b.Replace && !b.Final) {
		// A provider's first batch belongs to the new invocation. It is authoritative even
		// when empty; subsequent batches extend its streamed answer.
		s.items = nil
		s.refreshing = false
	}
	if b.Replace {
		s.items = nil
	}
	s.items = append(s.items, c.learned(b.Items, c.query)...)
	s.pending = s.pending || b.Pending
	if b.Final {
		c.finishLocked(s)
	}
	if c.opts.Synchronous {
		c.mu.Unlock()
		return
	}
	snap := c.snapshotLocked()
	c.mu.Unlock()
	c.publish(snap)
}

// complete marks a provider's call as returned, whatever it did. It runs after the call, panics included. A call that
// was cut off has already settled its slot; returning only ends what the slot still accepts and releases its context.
func (c *Coordinator) complete(gen, idx int, returned bool) {
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	s.returned = true
	if s.cancel != nil {
		s.cancel()
	}
	if s.finished {
		c.mu.Unlock()
		return
	}
	if s.refreshing && returned {
		// A provider that returned without emitting has answered with an empty result.
		s.items = nil
		s.refreshing = false
	}
	c.finishLocked(s)
	snap := c.snapshotLocked()
	c.mu.Unlock()
	c.publish(snap)
}

// acceptsLocked reports whether a batch of generation gen for slot idx still merges. A batch of an older query, or one
// that arrives after its provider returned, is dropped. A provider cut off as slow still merges until it returns.
func (c *Coordinator) acceptsLocked(gen, idx int) bool {
	if gen != c.gen || idx >= len(c.slots) {
		return false
	}
	s := c.slots[idx]
	return !s.returned && ((s.slow && s.selfBudgeted) || !s.finished)
}

// cutOff ends the wait for a call that ran past the hard budget: its rows so far are kept, it is counted as slow, and
// the merge does not wait for it. Only an aggregate that owns individual call deadlines keeps running to merge late results.
func (c *Coordinator) cutOff(gen, idx int) {
	c.mu.Lock()
	if gen != c.gen || idx >= len(c.slots) || c.slots[idx].finished {
		c.mu.Unlock()
		return
	}
	s := c.slots[idx]
	s.slow = true
	c.statsLocked(s.id).Slow++
	c.finishLocked(s)
	if !s.selfBudgeted && s.cancel != nil {
		s.cancel()
	}
	snap := c.snapshotLocked()
	c.mu.Unlock()
	logx.Default().Warn("search provider cut off at its hard budget", logx.F{"provider": s.id, "hard_ms": s.budget.Hard.Milliseconds()})
	c.publish(snap)
}

// finishLocked marks a slot finished and releases the goroutines waiting on it. The caller holds c.mu.
func (c *Coordinator) finishLocked(s *slot) {
	if s.finished {
		return
	}
	s.finished = true
	s.pending = false
	close(s.done)
	c.pending--
	elapsed := c.opts.Clock.Now().Sub(s.started)
	c.statsLocked(s.id).Last = elapsed
}
