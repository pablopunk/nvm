package fake

import (
	"errors"
	"io/fs"
	"path"
)

// Walk visits root and everything under it in lexical order, honoring fs.SkipDir and fs.SkipAll as
// filepath.WalkDir does. The callback runs without the lock held, so it may call other methods.
func (f *FS) Walk(root string, fn fs.WalkDirFunc) error {
	if err := f.faults.take(OpWalk); err != nil {
		return fn(root, nil, err)
	}
	key := cleanKey(root)
	f.mu.Lock()
	n, err := f.lookupLocked(key)
	f.mu.Unlock()
	if err != nil {
		return fn(root, nil, err)
	}
	err = f.walkNode(key, fileInfo{name: path.Base(key), node: n}, fn)
	if errors.Is(err, fs.SkipDir) || errors.Is(err, fs.SkipAll) {
		return nil
	}
	return err
}

// walkNode visits one node and, for folders, its children. SkipDir from a folder skips its contents;
// SkipDir from a file skips the rest of its folder; SkipAll propagates to the top. This is the contract of
// filepath.WalkDir.
func (f *FS) walkNode(key string, info fileInfo, fn fs.WalkDirFunc) error {
	err := fn(key, fs.FileInfoToDirEntry(info), nil)
	if err != nil || !info.IsDir() {
		if errors.Is(err, fs.SkipDir) && info.IsDir() {
			return nil
		}
		return err
	}
	f.mu.Lock()
	children := f.childrenLocked(key)
	f.mu.Unlock()
	for _, child := range children {
		f.mu.Lock()
		childNode, ok := f.nodes[child]
		f.mu.Unlock()
		if !ok {
			continue
		}
		if err := f.walkNode(child, fileInfo{name: path.Base(child), node: childNode}, fn); err != nil {
			if errors.Is(err, fs.SkipDir) {
				break
			}
			return err
		}
	}
	return nil
}
