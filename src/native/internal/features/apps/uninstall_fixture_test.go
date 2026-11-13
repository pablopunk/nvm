package apps

import (
	"errors"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// memTree is an in-memory Files for the uninstall tests. Every item has an owner and an inode; a path replaced with
// replace() keeps its name with a new inode, which is how a changed item is simulated. Trash removes the subtree
// and records it, so the tests can assert what would have moved, in order.
type memTree struct {
	nodes       map[string]*memNode
	nextIno     uint64
	trashed     []string
	failTrashAt string
	onLstat     func(p string)
}

type memNode struct {
	dir     bool
	symlink bool
	uid     int
	ino     uint64
	mode    fs.FileMode
	content []byte
}

func newMemTree() *memTree {
	m := &memTree{nodes: map[string]*memNode{}, nextIno: 100}
	m.nodes["/"] = &memNode{dir: true, uid: 0, ino: 1, mode: fs.ModeDir | 0o755}
	return m
}

// ensureParents creates the folders above p that do not exist yet, owned by uid.
func (m *memTree) ensureParents(p string, uid int) {
	var current string
	for _, part := range strings.Split(strings.TrimPrefix(path.Dir(p), "/"), "/") {
		if part == "" {
			continue
		}
		current += "/" + part
		if _, ok := m.nodes[current]; !ok {
			m.add(current, &memNode{dir: true, uid: uid, mode: fs.ModeDir | 0o755})
		}
	}
}

func (m *memTree) add(p string, n *memNode) {
	m.nextIno++
	n.ino = m.nextIno
	m.nodes[p] = n
}

// dir adds a folder owned by uid, creating its parents as the same user.
func (m *memTree) dir(p string, uid int) {
	m.ensureParents(p, uid)
	m.add(path.Clean(p), &memNode{dir: true, uid: uid, mode: fs.ModeDir | 0o755})
}

// file adds a regular file owned by uid with the given permission bits.
func (m *memTree) file(p string, uid int, mode fs.FileMode, content []byte) {
	m.ensureParents(p, uid)
	m.add(path.Clean(p), &memNode{uid: uid, mode: mode, content: content})
}

// link adds a symbolic link.
func (m *memTree) link(p string, uid int) {
	m.ensureParents(p, uid)
	m.add(path.Clean(p), &memNode{symlink: true, uid: uid, mode: fs.ModeSymlink | 0o777})
}

// replace gives the item at p a new inode, as a file that was deleted and recreated would have.
func (m *memTree) replace(p string) {
	if n, ok := m.nodes[p]; ok {
		m.nextIno++
		n.ino = m.nextIno
	}
}

// has reports whether an item exists at p.
func (m *memTree) has(p string) bool {
	_, ok := m.nodes[p]
	return ok
}

// remove deletes p and everything under it.
func (m *memTree) remove(p string) {
	for key := range m.nodes {
		if key == p || strings.HasPrefix(key, p+"/") {
			delete(m.nodes, key)
		}
	}
}

func missingErr(name string) error {
	return &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
}

// Lstat describes the item at name; a missing item is an fs.ErrNotExist error.
func (m *memTree) Lstat(name string) (Entry, error) {
	name = path.Clean(name)
	if m.onLstat != nil {
		m.onLstat(name)
	}
	n, ok := m.nodes[name]
	if !ok {
		return Entry{}, missingErr(name)
	}
	return Entry{
		Dir:      n.dir,
		Symlink:  n.symlink,
		Mode:     n.mode,
		Identity: Identity{Dir: n.dir, Dev: 1, Ino: n.ino, UID: n.uid},
	}, nil
}

// ReadDir lists the direct children of a folder, sorted by name.
func (m *memTree) ReadDir(name string) ([]DirEntry, error) {
	name = path.Clean(name)
	n, ok := m.nodes[name]
	if !ok {
		return nil, missingErr(name)
	}
	if !n.dir {
		return nil, errors.New("not a directory")
	}
	var out []DirEntry
	for key, child := range m.nodes {
		if key != name && path.Dir(key) == name {
			out = append(out, DirEntry{Name: path.Base(key), Dir: child.dir, Symlink: child.symlink})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Tree counts name and everything under it, failing with errTreeTooLarge past limit.
func (m *memTree) Tree(name string, limit int) (int, error) {
	name = path.Clean(name)
	count := 0
	for key := range m.nodes {
		if key == name || strings.HasPrefix(key, name+"/") {
			count++
		}
	}
	if count > limit {
		return count, errTreeTooLarge
	}
	return count, nil
}

// ReadFile returns the content of a regular file.
func (m *memTree) ReadFile(name string) ([]byte, error) {
	n, ok := m.nodes[path.Clean(name)]
	if !ok {
		return nil, missingErr(name)
	}
	return n.content, nil
}

// trash is the Trash port of the tree: it removes the item and records it, or fails for failTrashAt.
func (m *memTree) trash(p string) error {
	if p == m.failTrashAt {
		return errors.New("Trash denied")
	}
	if !m.has(p) {
		return missingErr(p)
	}
	m.trashed = append(m.trashed, p)
	m.remove(p)
	return nil
}

// fakeDirectoryAccess models an OS access decision independently of ownership.
type fakeDirectoryAccess struct{ tree *memTree }

func (f fakeDirectoryAccess) CanWrite(p string) bool {
	entry, err := f.tree.Lstat(p)
	return err == nil && entry.Dir && entry.Mode.Perm()&0o300 == 0o300
}
