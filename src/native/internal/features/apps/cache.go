package apps

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/store"
)

// SnapshotVersion is the schema version the index writes. A snapshot with a higher version came from a newer
// build: the index loads nothing from it and never overwrites it.
const SnapshotVersion = 1

// Snapshot is the persisted index: the last good app list and the first-seen state.
type Snapshot struct {
	Version   int       `json:"version"`
	Apps      []App     `json:"apps"`
	FirstSeen FirstSeen `json:"firstSeen"`
}

// Cache loads and saves the snapshot. The store adapter (NewCollectionCache) writes apps-cache.json atomically;
// the index depends only on this interface.
type Cache interface {
	// Load returns the saved snapshot. A missing snapshot is the zero value and a nil error.
	Load() (Snapshot, error)
	// Save replaces the saved snapshot.
	Save(snapshot Snapshot) error
}

// CacheDocumentName is the store file that holds the snapshot.
const CacheDocumentName = "apps-cache.json"

// NewCacheDocument registers the apps cache in s. The composition root calls it once per store.
func NewCacheDocument(s *store.Store) *store.Collection[Snapshot] {
	return store.Doc(s, CacheDocumentName, SnapshotVersion, func() Snapshot {
		return Snapshot{Version: SnapshotVersion, FirstSeen: FirstSeen{ByID: map[string]int64{}}}
	})
}

type collectionCache struct {
	doc *store.Collection[Snapshot]
}

// NewCollectionCache returns a Cache over a store document registered with NewCacheDocument.
func NewCollectionCache(doc *store.Collection[Snapshot]) Cache {
	return collectionCache{doc: doc}
}

// Load returns the stored snapshot.
func (c collectionCache) Load() (Snapshot, error) {
	return c.doc.Load()
}

// Save stores the snapshot and flushes it, so a crash right after a scan keeps the list.
func (c collectionCache) Save(snapshot Snapshot) error {
	if err := c.doc.Update(func(current *Snapshot) { *current = snapshot }); err != nil {
		return err
	}
	return c.doc.Flush()
}

// MemoryCache keeps the snapshot in memory, for tests and for profiles that must not touch disk.
type MemoryCache struct {
	mu       sync.Mutex
	snapshot Snapshot
	saves    int
	failSave error
}

var _ Cache = (*MemoryCache)(nil)

// Load returns a copy of the stored snapshot.
func (m *MemoryCache) Load() (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneSnapshot(m.snapshot), nil
}

// Save stores a copy of snapshot, or returns the error set by FailNextSave once.
func (m *MemoryCache) Save(snapshot Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failSave != nil {
		err := m.failSave
		m.failSave = nil
		return err
	}
	m.snapshot = cloneSnapshot(snapshot)
	m.saves++
	return nil
}

// FailNextSave makes the next Save return err without storing anything.
func (m *MemoryCache) FailNextSave(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failSave = err
}

// Saves reports how many snapshots were stored.
func (m *MemoryCache) Saves() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saves
}

func cloneSnapshot(s Snapshot) Snapshot {
	return Snapshot{Version: s.Version, Apps: append([]App(nil), s.Apps...), FirstSeen: cloneFirstSeen(s.FirstSeen)}
}

func cloneFirstSeen(state FirstSeen) FirstSeen {
	out := FirstSeen{Initialized: state.Initialized, ByID: make(map[string]int64, len(state.ByID))}
	for id, at := range state.ByID {
		out.ByID[id] = at
	}
	return out
}
