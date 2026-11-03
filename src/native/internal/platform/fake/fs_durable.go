package fake

import (
	"bytes"
	"io/fs"
	"path"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by FS.FailNext for the durability methods.
const (
	OpCreateExclusive = "CreateExclusive"
	OpSync            = "Sync"
)

// CreateExclusive creates a file only when name is free. A taken name returns an error matching
// fs.ErrExist and the existing file is left untouched.
func (f *FS) CreateExclusive(name string, data []byte, perm fs.FileMode) error {
	if err := f.faults.take(OpCreateExclusive); err != nil {
		return pathError("open", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	parent, err := f.lookupLocked(path.Dir(key))
	if err != nil || !parent.dir {
		return pathError("open", name, fs.ErrNotExist)
	}
	if _, exists := f.nodes[key]; exists {
		return pathError("open", name, fs.ErrExist)
	}
	f.nodes[key] = &node{mode: perm.Perm(), data: bytes.Clone(data)}
	f.kids = nil
	f.notifyLocked(platform.Event{Op: platform.EventCreate, Path: key})
	return nil
}

// Sync succeeds for any existing file or folder. The fake keeps everything in memory, so there is nothing
// to flush; the call exists so code under test takes the same path it takes on disk.
func (f *FS) Sync(name string) error {
	if err := f.faults.take(OpSync); err != nil {
		return pathError("sync", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := f.lookupLocked(name); err != nil {
		return pathError("sync", name, err)
	}
	return nil
}
