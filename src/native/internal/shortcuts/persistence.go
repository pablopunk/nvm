package shortcuts

import "sync"

// Persistence is where the document lives: a store collection in the app, a MemoryDocument when the store is not
// available or in tests.
type Persistence interface {
	Load() (Document, error)
	Update(change func(*Document)) error
}

// MemoryDocument keeps the document in memory. Changes last as long as the value does.
type MemoryDocument struct {
	mu  sync.Mutex
	doc Document
}

// NewMemoryDocument returns a memory document that starts as doc.
func NewMemoryDocument(doc Document) *MemoryDocument {
	return &MemoryDocument{doc: doc.clone()}
}

// Load returns a copy of the document.
func (m *MemoryDocument) Load() (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doc.clone(), nil
}

// Update applies change to the document.
func (m *MemoryDocument) Update(change func(*Document)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	change(&m.doc)
	return nil
}
