package clipboard

import (
	"slices"
	"sync"
)

// SnapshotVersion is the schema version History writes. A snapshot with a higher version was written by a
// newer build: History loads nothing from it and stays read-only, so the newer data is never overwritten.
const SnapshotVersion = 1

// Snapshot is the versioned document History persists: every entry, newest first.
type Snapshot struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

// Persistence loads and saves the history snapshot. The store package adapter writes the real file
// atomically and keeps good copies; History depends only on this interface.
type Persistence interface {
	// Load returns the saved snapshot. A missing snapshot is the zero Snapshot and a nil error.
	Load() (Snapshot, error)
	// Save replaces the saved snapshot.
	Save(snapshot Snapshot) error
}

// MemoryPersistence keeps the snapshot in memory. It serves tests and profiles that must not touch disk.
type MemoryPersistence struct {
	mu       sync.Mutex
	snapshot Snapshot
	saves    int
	failSave error
}

var _ Persistence = (*MemoryPersistence)(nil)

// NewMemoryPersistence returns an empty in-memory store.
func NewMemoryPersistence() *MemoryPersistence {
	return &MemoryPersistence{}
}

// Load returns a copy of the saved snapshot.
func (m *MemoryPersistence) Load() (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneSnapshot(m.snapshot), nil
}

// Save stores a copy of snapshot, or returns the error set by FailNextSave once.
func (m *MemoryPersistence) Save(snapshot Snapshot) error {
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
func (m *MemoryPersistence) FailNextSave(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failSave = err
}

// Saves reports how many snapshots were stored.
func (m *MemoryPersistence) Saves() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saves
}

// Stored returns a copy of the stored snapshot, for assertions.
func (m *MemoryPersistence) Stored() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneSnapshot(m.snapshot)
}

// SetStored replaces the stored snapshot without counting as a save. Tests use it to seed a history.
func (m *MemoryPersistence) SetStored(snapshot Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshot = cloneSnapshot(snapshot)
}

// cloneSnapshot copies the entry list. Entries share their path slices, which History never mutates.
func cloneSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{Version: snapshot.Version, Entries: slices.Clone(snapshot.Entries)}
}
