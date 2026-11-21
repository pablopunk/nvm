package store

import (
	"errors"
	"fmt"
)

// ErrPendingChanges means Commit cannot safely replace an unflushed Update.
var ErrPendingChanges = errors.New("store: document has pending changes")

// Commit applies change to a private copy and publishes it only after the atomic save succeeds.
// The callback runs under the collection lock and may reject a stale revision. It must not retain
// the supplied pointer or call back into this collection. Commit refuses pending debounced updates.
func (c *Collection[T]) Commit(change func(*T) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.loadLocked()
	}
	if err := c.writableLocked(); err != nil {
		return err
	}
	if c.dirty {
		return ErrPendingChanges
	}
	candidate, err := c.snapshotLocked()
	if err != nil {
		return err
	}
	if err := change(&candidate); err != nil {
		return err
	}
	previous := c.value
	c.value = candidate
	isolated, err := c.snapshotLocked()
	if err != nil {
		c.value = previous
		return err
	}
	c.value = isolated
	_, omitted, err := c.encodeLocked()
	if err != nil {
		c.value = previous
		return fmt.Errorf("store: encode %s: %w", c.name, err)
	}
	if len(omitted) != 0 {
		c.value = previous
		return ErrSecretInStore
	}
	if err := c.saveLocked(); err != nil {
		c.value = previous
		return err
	}
	c.notifyLocked()
	return nil
}
