package fake

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by FS.FailNext.
const (
	OpReadFile  = "ReadFile"
	OpWriteFile = "WriteFile"
	OpRename    = "Rename"
	OpRemove    = "Remove"
	OpMkdirAll  = "MkdirAll"
	OpStat      = "Stat"
	OpReadDir   = "ReadDir"
	OpWalk      = "Walk"
	OpChmod     = "Chmod"
	OpWatch     = "Watch"
)

// node is one file, folder or symbolic link of the in-memory tree. A link has link set to the absolute
// key of its target and no content.
type node struct {
	dir   bool
	mode  fs.FileMode
	data  []byte
	mtime time.Time
	link  string
}

// FS is an in-memory platform.FS. Paths are slash-separated and rooted at "/"; a relative name is taken
// relative to the root. Errors wrap fs.ErrNotExist, fs.ErrExist and friends so errors.Is works as it does
// for the OS. ModTime is the zero time unless SetModTime sets it; SeedSymlink models a link.
//
// Fault injection: FailNext makes the next call of an operation fail before it changes anything.
// AfterWrite runs a hook after a WriteFile has been applied; the hook's error is returned to the caller
// while the data stays, which is how a crash between the write and the acknowledgement looks.
//
//	fs := fake.NewFS()
//	fs.FailNext(fake.OpWriteFile, errors.New("disk full"))
//	err := fs.WriteFile("/cfg/settings.json", data, 0o600) // err, nothing written
type FS struct {
	mu         sync.Mutex
	nodes      map[string]*node
	faults     faultSet
	afterWrite func(name string) error
	watchers   []*fsWatcher
}

var _ platform.FS = (*FS)(nil)

// NewFS returns an empty file system with only the root folder.
func NewFS() *FS {
	return &FS{nodes: map[string]*node{"/": {dir: true, mode: fs.ModeDir | 0o755}}}
}

// FailNext makes the next call of op return err without changing the tree.
func (f *FS) FailNext(op string, err error) {
	f.faults.failNext(op, err)
}

// AfterWrite installs a hook that runs after each successful WriteFile, outside the file system lock.
// Passing nil removes it.
func (f *FS) AfterWrite(hook func(name string) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.afterWrite = hook
}

// Seed creates or replaces a file with data, creating parent folders. It bypasses faults and watchers;
// use it to set up fixtures before the code under test runs. It panics when a parent is a file.
func (f *FS) Seed(name string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	if err := f.mkdirAllLocked(path.Dir(key)); err != nil {
		panic("fake: seeding " + name + ": " + err.Error())
	}
	f.nodes[key] = &node{mode: 0o644, data: bytes.Clone(data)}
}

// Paths lists every path in the tree, sorted, including folders. Tests use it for assertions.
func (f *FS) Paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	paths := make([]string, 0, len(f.nodes))
	for name := range f.nodes {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	return paths
}

// ReadFile returns a copy of the file's content.
func (f *FS) ReadFile(name string) ([]byte, error) {
	if err := f.faults.take(OpReadFile); err != nil {
		return nil, pathError("open", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.lookupLocked(name)
	if err != nil {
		return nil, pathError("open", name, err)
	}
	if n.dir {
		return nil, pathError("read", name, errors.New("is a directory"))
	}
	return bytes.Clone(n.data), nil
}

// WriteFile replaces a file's content. The parent folder must exist.
func (f *FS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.faults.take(OpWriteFile); err != nil {
		return pathError("open", name, err)
	}
	f.mu.Lock()
	key := cleanKey(name)
	parent, err := f.lookupLocked(path.Dir(key))
	if err != nil || !parent.dir {
		f.mu.Unlock()
		return pathError("open", name, fs.ErrNotExist)
	}
	existing, exists := f.nodes[key]
	if exists && existing.dir {
		f.mu.Unlock()
		return pathError("open", name, errors.New("is a directory"))
	}
	f.nodes[key] = &node{mode: perm.Perm(), data: bytes.Clone(data)}
	op := platform.EventWrite
	if !exists {
		op = platform.EventCreate
	}
	hook := f.afterWrite
	f.notifyLocked(platform.Event{Op: op, Path: key})
	f.mu.Unlock()

	if hook != nil {
		return hook(name)
	}
	return nil
}

// Rename moves a file or folder, replacing a file at the destination.
func (f *FS) Rename(oldName, newName string) error {
	if err := f.faults.take(OpRename); err != nil {
		return pathError("rename", oldName, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	from, to := cleanKey(oldName), cleanKey(newName)
	if _, err := f.lookupLocked(from); err != nil {
		return pathError("rename", oldName, err)
	}
	if parent, err := f.lookupLocked(path.Dir(to)); err != nil || !parent.dir {
		return pathError("rename", newName, fs.ErrNotExist)
	}
	if existing, ok := f.nodes[to]; ok && existing.dir {
		return pathError("rename", newName, errors.New("is a directory"))
	}
	f.moveSubtreeLocked(from, to)
	f.notifyLocked(platform.Event{Op: platform.EventRename, Path: to, OldPath: from, IsDir: f.nodes[to].dir})
	return nil
}

// Remove deletes a file or an empty folder. The root cannot be removed.
func (f *FS) Remove(name string) error {
	if err := f.faults.take(OpRemove); err != nil {
		return pathError("remove", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	n, err := f.lookupLocked(key)
	if err != nil {
		return pathError("remove", name, err)
	}
	if key == "/" {
		return pathError("remove", name, errors.New("cannot remove the root"))
	}
	if n.dir && f.hasChildLocked(key) {
		return pathError("remove", name, errors.New("directory not empty"))
	}
	delete(f.nodes, key)
	f.notifyLocked(platform.Event{Op: platform.EventRemove, Path: key, IsDir: n.dir})
	return nil
}

// MkdirAll creates a folder and its parents. Existing files on the way are an error.
func (f *FS) MkdirAll(name string, perm fs.FileMode) error {
	if err := f.faults.take(OpMkdirAll); err != nil {
		return pathError("mkdir", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mkdirAllLocked(cleanKey(name))
}

// Stat describes a file or folder, following a symbolic link once to its target.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	if err := f.faults.take(OpStat); err != nil {
		return nil, pathError("stat", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	n, err := f.lookupLocked(key)
	if err != nil {
		return nil, pathError("stat", name, err)
	}
	if n.link != "" {
		target, err := f.lookupLocked(n.link)
		if err != nil {
			return nil, pathError("stat", name, err)
		}
		n = target
	}
	return fileInfo{name: path.Base(key), node: n}, nil
}

// Lstat describes a file or folder without following a symbolic link.
func (f *FS) Lstat(name string) (fs.FileInfo, error) {
	if err := f.faults.take(OpStat); err != nil {
		return nil, pathError("lstat", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	n, err := f.lookupLocked(key)
	if err != nil {
		return nil, pathError("lstat", name, err)
	}
	return fileInfo{name: path.Base(key), node: n}, nil
}

// SeedSymlink creates or replaces a symbolic link at name pointing to target, creating parent folders.
// It bypasses faults and watchers, like Seed. Stat follows the link; Lstat and ReadDir report the link.
func (f *FS) SeedSymlink(name, target string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	if err := f.mkdirAllLocked(path.Dir(key)); err != nil {
		panic("fake: seeding link " + name + ": " + err.Error())
	}
	f.nodes[key] = &node{mode: fs.ModeSymlink | 0o777, link: cleanKey(target)}
}

// SetModTime sets the modification time reported for name by Stat, Lstat and ReadDir. It panics when name
// does not exist, because a test that sets the time of a missing file is wrong.
func (f *FS) SetModTime(name string, mtime time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nodes[cleanKey(name)]
	if !ok {
		panic("fake: set mod time of missing " + name)
	}
	n.mtime = mtime
}

// ReadDir lists a folder's entries sorted by name.
func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.faults.take(OpReadDir); err != nil {
		return nil, pathError("open", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	n, err := f.lookupLocked(key)
	if err != nil {
		return nil, pathError("open", name, err)
	}
	if !n.dir {
		return nil, pathError("readdir", name, errors.New("not a directory"))
	}
	var entries []fs.DirEntry
	for _, child := range f.childrenLocked(key) {
		entries = append(entries, fs.FileInfoToDirEntry(fileInfo{name: path.Base(child), node: f.nodes[child]}))
	}
	return entries, nil
}

// Chmod changes the permission bits of a file or folder.
func (f *FS) Chmod(name string, mode fs.FileMode) error {
	if err := f.faults.take(OpChmod); err != nil {
		return pathError("chmod", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.lookupLocked(name)
	if err != nil {
		return pathError("chmod", name, err)
	}
	n.mode = mode.Perm()
	return nil
}

// Watch reports changes at or under name until ctx is done, then closes the channel. Events that do not
// fit the buffer are dropped, as the port allows.
func (f *FS) Watch(ctx context.Context, name string) (<-chan platform.Event, error) {
	if err := f.faults.take(OpWatch); err != nil {
		return nil, pathError("watch", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	if _, err := f.lookupLocked(key); err != nil {
		return nil, pathError("watch", name, err)
	}
	w := &fsWatcher{prefix: key, out: make(chan platform.Event, watcherBuffer)}
	f.watchers = append(f.watchers, w)
	go f.unwatchOnDone(ctx, w)
	return w.out, nil
}

// unwatchOnDone detaches a watcher and closes its channel when ctx ends.
func (f *FS) unwatchOnDone(ctx context.Context, w *fsWatcher) {
	<-ctx.Done()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watchers = slices.DeleteFunc(f.watchers, func(other *fsWatcher) bool { return other == w })
	close(w.out)
}

// pathError wraps err the way os does, so callers can inspect Op, Path and Err.
func pathError(op, name string, err error) error {
	return &fs.PathError{Op: op, Path: name, Err: err}
}
