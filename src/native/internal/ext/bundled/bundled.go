// Package bundled serves the read-only datasets the app ships with itself, such as the emoji catalog. A dataset is a
// JSON file embedded in the binary; the first read decodes it and every later read returns the same records.
//
// Layer: ext. The package imports api and the standard library.
package bundled

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// shippedFiles holds the datasets the app ships. Each file is named after its dataset, <name>.json.
var shippedFiles = mustSub(embedded, "datasets")

//go:embed datasets/*.json
var embedded embed.FS

// mustSub is the datasets folder of the embedded files, so names resolve from its root.
func mustSub(files embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(files, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// Store answers reads of named datasets. Each dataset is decoded at most once, on its first read.
type Store struct {
	files    fs.FS
	mu       sync.Mutex
	datasets map[string]*dataset
}

// dataset is one decoded file. The once guards the decode, and records or err are its result.
type dataset struct {
	once    sync.Once
	records []api.BundleRecord
	err     error
}

// Shipped returns the store of the datasets embedded in the app.
func Shipped() *Store {
	return NewStore(shippedFiles)
}

// NewStore returns a store that reads its datasets from files, one <name>.json file each under the root of files.
func NewStore(files fs.FS) *Store {
	return &Store{files: files, datasets: map[string]*dataset{}}
}

// Records returns the records of the named dataset. A name that is not a dataset fails with ErrUnknownDataset, which
// the services layer reports as unavailable.
func (s *Store) Records(name string) ([]api.BundleRecord, error) {
	entry := s.entry(name)
	entry.once.Do(func() { entry.records, entry.err = s.decode(name) })
	return entry.records, entry.err
}

// entry returns the cache slot of a dataset name, creating it on first use.
func (s *Store) entry(name string) *dataset {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.datasets[name]
	if !ok {
		entry = &dataset{}
		s.datasets[name] = entry
	}
	return entry
}

// decode reads and parses the file of a dataset.
func (s *Store) decode(name string) ([]api.BundleRecord, error) {
	if !validName(name) {
		return nil, ErrUnknownDataset
	}
	raw, err := fs.ReadFile(s.files, name+".json")
	if err != nil {
		return nil, ErrUnknownDataset
	}
	var records []api.BundleRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("bundled dataset %q is not valid: %w", name, err)
	}
	return records, nil
}

// ErrUnknownDataset is returned for a name the app does not ship.
var ErrUnknownDataset = fmt.Errorf("%w: no such bundled dataset", api.ErrNotAvailable)

// validName keeps a dataset name to a plain file name, so a name cannot reach outside the datasets folder.
func validName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !(c == '-' || c == '_' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
