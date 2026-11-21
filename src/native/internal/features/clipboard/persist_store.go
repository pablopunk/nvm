package clipboard

import (
	"github.com/pablopunk/nvm/native/internal/store"
)

// SnapshotDocument is the store document that holds the clipboard history. Its schema version is the store's
// envelope version; the Snapshot inside carries SnapshotVersion as well.
const (
	SnapshotDocument      = "clipboard.json"
	SnapshotSchemaVersion = 1
)

// NewSnapshotDocument registers the history document in s. Missing files load as an empty snapshot.
func NewSnapshotDocument(s *store.Store) *store.Collection[Snapshot] {
	return store.Doc(s, SnapshotDocument, SnapshotSchemaVersion, emptySnapshot)
}

// emptySnapshot is the value of a missing history document.
func emptySnapshot() Snapshot {
	return Snapshot{Version: SnapshotVersion}
}

// storePersistence adapts a store document to Persistence. The store writes atomically, debounces writes,
// keeps good copies and quarantines a corrupt file; this adapter only maps the calls.
type storePersistence struct {
	doc *store.Collection[Snapshot]
}

// NewStorePersistence returns the Persistence of the history document.
func NewStorePersistence(doc *store.Collection[Snapshot]) Persistence {
	return storePersistence{doc: doc}
}

// Load returns the saved snapshot. A document the store opened read-only (written by a newer build, or
// unreadable) is reported as a newer snapshot, so History opens read-only and never overwrites it. An
// unreadable document loads as empty and is treated the same way.
func (p storePersistence) Load() (Snapshot, error) {
	snapshot, err := p.doc.Load()
	if err != nil || p.doc.ReadOnly() {
		snapshot.Version = SnapshotVersion + 1
	}
	return snapshot, nil
}

// Save replaces the document's value. The store writes it after the debounce, and on Flush at quit.
func (p storePersistence) Save(snapshot Snapshot) error {
	return p.doc.Update(func(value *Snapshot) {
		*value = Snapshot{Version: SnapshotVersion, Entries: snapshot.Entries}
	})
}
