package fake

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// cleanKey turns a name into the tree key: slash-separated, cleaned, rooted at "/".
func cleanKey(name string) string {
	return path.Clean("/" + strings.ReplaceAll(name, "\\", "/"))
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

// childrenLocked lists the direct children of key, sorted. The caller holds f.mu.
func (f *FS) childrenLocked(key string) []string {
	prefix := key + "/"
	if key == "/" {
		prefix = "/"
	}
	var children []string
	for name := range f.nodes {
		if name == key || !strings.HasPrefix(name, prefix) || strings.Contains(strings.TrimPrefix(name, prefix), "/") {
			continue
		}
		children = append(children, name)
	}
	slices.Sort(children)
	return children
}
