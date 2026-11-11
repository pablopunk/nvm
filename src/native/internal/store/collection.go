package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Collection is one typed JSON document of a store. Load returns copies, Update changes the document under
// its lock, and writes are debounced. A Collection is safe for concurrent use. T must round-trip through
// encoding/json; its unexported fields are not persisted.
type Collection[T any] struct {
	store       *Store
	name        string
	path        string
	tmpPath     string
	version     int
	defaults    func() T
	migrations  []Migration
	secrets     []int
	keepBackups int

	mu          sync.Mutex
	value       T
	loaded      bool
	loadErr     error
	readOnly    bool
	dirty       bool
	waiting     bool
	deadline    time.Time
	onDisk      []byte
	subscribers []chan struct{}
}

// Doc registers a typed document named name (a base file name such as "settings.json") in s. schemaVersion
// is the version this build writes, starting at 1. defaults supplies the value of a missing document and
// the base that stored fields are decoded over. migrations must hold one step per version before
// schemaVersion, in order: migrations[i].From == i+1. Misconfiguration panics, because it is a programming
// error that tests catch at once.
func Doc[T any](s *Store, name string, schemaVersion int, defaults func() T, migrations ...Migration) *Collection[T] {
	validateDocumentName(name)
	if schemaVersion < 1 {
		panic(fmt.Sprintf("store: %s: schemaVersion must be at least 1", name))
	}
	if defaults == nil {
		panic(fmt.Sprintf("store: %s: defaults is required", name))
	}
	validateMigrations(name, schemaVersion, migrations)
	collection := &Collection[T]{
		store:       s,
		name:        name,
		path:        filepath.Join(s.dir, name),
		tmpPath:     filepath.Join(s.dir, "."+name+".tmp"),
		version:     schemaVersion,
		defaults:    defaults,
		migrations:  slices.Clone(migrations),
		secrets:     secretFieldIndexes[T](),
		keepBackups: s.cfg.keepBackups,
	}
	s.attach(name, collection)
	return collection
}

// WithoutBackups disables new backup copies for this document. Call immediately after Doc,
// before the document is loaded or used. Existing copies are never deleted.
func (c *Collection[T]) WithoutBackups() *Collection[T] {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		panic("store: backup policy must be set before loading")
	}
	c.keepBackups = 0
	return c
}

// validateDocumentName panics unless name is a plain file name.
func validateDocumentName(name string) {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		panic(fmt.Sprintf("store: document name %q must be a plain file name", name))
	}
}

// Path returns the file the document is stored in.
func (c *Collection[T]) Path() string {
	return c.path
}

// Load returns a deep copy of the document. The first call reads the file: a corrupt file is quarantined,
// an older one is migrated, and a missing one yields defaults without a write. The error reports a read or
// migration failure; the value is then the defaults and the document is read-only.
func (c *Collection[T]) Load() (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.loadLocked()
	}
	snapshot, err := c.snapshotLocked()
	if err != nil {
		return snapshot, err
	}
	return snapshot, c.loadErr
}

// ReadOnly reports whether the document refuses writes: a newer build wrote the file, or it could not be read.
// It loads the document first when that has not happened yet.
func (c *Collection[T]) ReadOnly() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.loadLocked()
	}
	return c.readOnly
}

// Update applies change to the document under its lock, then schedules a debounced write and a change
// event. It returns ErrClosed, ErrReadOnly or ErrLeaseLost when the document cannot take changes, and then
// change is not called.
func (c *Collection[T]) Update(change func(*T)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.loadLocked()
	}
	if err := c.writableLocked(); err != nil {
		return err
	}
	change(&c.value)
	c.dirty = true
	c.scheduleLocked()
	c.notifyLocked()
	return nil
}

// Save writes the document now when its content differs from the file on disk. A non-zero secret field
// is omitted and ErrSecretInStore is returned after the write.
func (c *Collection[T]) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loaded {
		c.loadLocked()
	}
	if err := c.writableLocked(); err != nil {
		return err
	}
	return c.saveLocked()
}

// Flush writes the document when it has unsaved changes and does nothing otherwise. Quit handlers call it.
func (c *Collection[T]) Flush() error {
	return c.flushNow()
}

// Subscribe returns a channel that receives a value after each in-memory change. The channel holds one
// pending signal, so a slow reader sees the latest state on its next read. It is closed when the store
// closes.
func (c *Collection[T]) Subscribe() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	channel := make(chan struct{}, 1)
	if c.store.isClosed() {
		close(channel)
		return channel
	}
	c.subscribers = append(c.subscribers, channel)
	return channel
}

// flushNow implements document for Store.Flush.
func (c *Collection[T]) flushNow() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	return c.saveLocked()
}

// closeDocument implements document for Store.Close: it writes pending changes, then closes subscriptions.
func (c *Collection[T]) closeDocument() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	if c.dirty {
		err = c.saveLocked()
	}
	for _, channel := range c.subscribers {
		close(channel)
	}
	c.subscribers = nil
	return err
}

// writableLocked reports why the document cannot take a change, or nil. The caller holds c.mu.
func (c *Collection[T]) writableLocked() error {
	if c.store.isClosed() {
		return ErrClosed
	}
	if c.readOnly {
		return ErrReadOnly
	}
	if lease := c.store.cfg.lease; lease != nil {
		return lease.Lost()
	}
	return nil
}

// notifyLocked signals subscribers after an in-memory change. The caller holds c.mu.
func (c *Collection[T]) notifyLocked() {
	for _, channel := range c.subscribers {
		select {
		case channel <- struct{}{}:
		default:
		}
	}
}

// snapshotLocked returns a deep copy of the in-memory value by a JSON round trip, which keeps secret fields
// so that the app can use them. The caller holds c.mu.
func (c *Collection[T]) snapshotLocked() (T, error) {
	var copied T
	data, err := json.Marshal(c.value)
	if err != nil {
		return copied, fmt.Errorf("store: copy %s: %w", c.name, err)
	}
	if err := json.Unmarshal(data, &copied); err != nil {
		return copied, fmt.Errorf("store: copy %s: %w", c.name, err)
	}
	return copied, nil
}

// validateMigrations panics unless migrations holds exactly one step per version before schemaVersion.
func validateMigrations(name string, schemaVersion int, migrations []Migration) {
	if len(migrations) != schemaVersion-1 {
		panic(fmt.Sprintf("store: %s: %d migrations for schema version %d, want %d", name, len(migrations), schemaVersion, schemaVersion-1))
	}
	for i, step := range migrations {
		if step.From != i+1 || step.Up == nil {
			panic(fmt.Sprintf("store: %s: migration %d must have From %d and an Up function", name, i, i+1))
		}
	}
}
