package fake

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// cleanKey turns a name into the tree key: slash-separated, cleaned, rooted at "/". The tree is one volume, so a
// drive letter is dropped and C:\home\me names the same node as /home/me on every host.
func cleanKey(name string) string {
	return path.Clean("/" + strings.ReplaceAll(withoutDrive(name), "\\", "/"))
}

// withoutDrive removes a leading drive letter and colon, which the tree does not model.
func withoutDrive(name string) string {
	if len(name) >= 2 && name[1] == ':' && (name[0] >= 'a' && name[0] <= 'z' || name[0] >= 'A' && name[0] <= 'Z') {
		return name[2:]
	}
	return name
}

// lookupLocked returns the node at name or an fs.ErrNotExist-wrapping error. The caller holds f.mu.
func (f *FS) lookupLocked(name string) (*node, error) {
	n, ok := f.nodes[cleanKey(name)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return n, nil
}

// mkdirAllLocked creates key and its parents, failing when a file is in the way. The caller holds f.mu.
func (f *FS) mkdirAllLocked(key string) error {
	current := ""
	for _, part := range strings.Split(strings.TrimPrefix(key, "/"), "/") {
		if part == "" {
			continue
		}
		current += "/" + part
		if existing, ok := f.nodes[current]; ok {
			if !existing.dir {
				return errors.New("platform fake: a file is in the way of " + key)
			}
			continue
		}
		f.nodes[current] = &node{dir: true, mode: fs.ModeDir | 0o755}
		f.kids = nil
	}
	return nil
}

// moveSubtreeLocked renames key from to to, along with everything under it. The caller holds f.mu.
func (f *FS) moveSubtreeLocked(from, to string) {
	var moving []string
	for key := range f.nodes {
		if key == from || strings.HasPrefix(key, from+"/") {
			moving = append(moving, key)
		}
	}
	for _, key := range moving {
		f.nodes[to+strings.TrimPrefix(key, from)] = f.nodes[key]
		delete(f.nodes, key)
	}
}

// hasChildLocked reports whether any node sits directly under key. The caller holds f.mu.
func (f *FS) hasChildLocked(key string) bool {
	return len(f.childrenLocked(key)) > 0
}

// childrenLocked lists the direct children of key, sorted. The caller holds f.mu. The child index is rebuilt
// on the first call after the set of paths changes, so a scan of a large tree costs one pass, not one per
// folder. Returned slices are never modified in place, so callers may keep them after unlocking.
func (f *FS) childrenLocked(key string) []string {
	if f.kids == nil {
		f.kids = f.buildChildIndexLocked()
	}
	return f.kids[key]
}

// buildChildIndexLocked maps every folder to its sorted direct children. The caller holds f.mu.
func (f *FS) buildChildIndexLocked() map[string][]string {
	kids := map[string][]string{}
	for name := range f.nodes {
		if name == "/" {
			continue
		}
		parent := path.Dir(name)
		kids[parent] = append(kids[parent], name)
	}
	for parent := range kids {
		slices.Sort(kids[parent])
	}
	return kids
}
