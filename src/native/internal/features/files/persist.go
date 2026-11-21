package files

import (
	"bytes"
	"io/fs"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Persistence keeps the encoded index snapshot between runs, so the next start can serve results before the first scan
// finishes. The bytes are versioned by the snapshot format itself; Load returns fs.ErrNotExist when nothing has been
// saved. Save replaces the previous snapshot as one step: a reader sees the old bytes or the new ones, never a mix.
type Persistence interface {
	Load() ([]byte, error)
	Save(data []byte) error
}

// MemoryPersistence keeps the snapshot in process memory. It serves tests and callers that do not want a file.
type MemoryPersistence struct {
	mu    sync.Mutex
	data  []byte
	saves int
}

// Load returns a copy of the stored bytes, or fs.ErrNotExist when none were saved.
func (m *MemoryPersistence) Load() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		return nil, fs.ErrNotExist
	}
	return bytes.Clone(m.data), nil
}

// Save stores a copy of data.
func (m *MemoryPersistence) Save(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = bytes.Clone(data)
	m.saves++
	return nil
}

// Saves reports how many times Save succeeded. Tests use it to check that a snapshot was written.
func (m *MemoryPersistence) Saves() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saves
}

// SetRaw replaces the stored bytes without counting a save. Tests use it to plant a corrupt snapshot.
func (m *MemoryPersistence) SetRaw(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = bytes.Clone(data)
}

// FilePersistence keeps the snapshot in one file. Saves write a sibling temporary file, flush it and rename it over
// the target, so an interrupted save leaves the previous file whole.
type FilePersistence struct {
	FS   platform.FS
	Path string
}

// Load reads the file. A missing file reports fs.ErrNotExist.
func (p FilePersistence) Load() ([]byte, error) {
	return p.FS.ReadFile(p.Path)
}

// Save writes data to a temporary file, flushes it and renames it over Path.
func (p FilePersistence) Save(data []byte) error {
	tmp := p.Path + ".tmp"
	if err := p.FS.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := p.FS.Sync(tmp); err != nil {
		return err
	}
	return p.FS.Rename(tmp, p.Path)
}
