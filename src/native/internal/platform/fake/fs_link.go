package fake

import (
	"io/fs"
	"path"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpSymlink is the operation name used by FailNext and FailPath for FS.Symlink.
const OpSymlink = "Symlink"

// Symlink creates a symbolic link name that points at target. The target is not checked and never followed by
// the fake: Stat and Lstat both describe the link itself, the way a walker that refuses to follow links sees
// it. The parent folder must exist and name must be free.
func (f *FS) Symlink(target, name string) error {
	if err := f.faults.take(OpSymlink); err != nil {
		return pathError("symlink", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	parent, err := f.lookupLocked(path.Dir(key))
	if err != nil || !parent.dir {
		return pathError("symlink", name, fs.ErrNotExist)
	}
	if _, exists := f.nodes[key]; exists {
		return pathError("symlink", name, fs.ErrExist)
	}
	f.nodes[key] = &node{mode: 0o777, link: target}
	f.kids = nil
	f.notifyLocked(platform.Event{Op: platform.EventCreate, Path: key})
	return nil
}
