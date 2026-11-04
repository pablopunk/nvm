package store

import "time"

// scheduleLocked moves the write deadline to one debounce period after this change. A single waiter
// goroutine serves each document: it is started by the first change, re-arms itself while the deadline
// keeps moving, and writes once the deadline passes. Many Updates therefore coalesce into one write. The
// caller holds c.mu.
func (c *Collection[T]) scheduleLocked() {
	debounce := c.store.cfg.debounce
	c.deadline = c.store.clock.Now().Add(debounce)
	if c.waiting {
		return
	}
	c.waiting = true
	go c.runDebounce(c.store.clock.After(debounce))
}

// runDebounce waits for the deadline and writes the document. It exits when the store closes, which
// Close then finishes with a synchronous flush.
func (c *Collection[T]) runDebounce(timer <-chan time.Time) {
	for {
		select {
		case <-c.store.done:
			return
		case <-timer:
		}
		c.mu.Lock()
		remaining := c.deadline.Sub(c.store.clock.Now())
		if remaining > 0 {
			timer = c.store.clock.After(remaining)
			c.mu.Unlock()
			continue
		}
		c.waiting = false
		if c.dirty {
			_ = c.saveLocked()
		}
		c.mu.Unlock()
		return
	}
}
