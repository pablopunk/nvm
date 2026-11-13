package fake

import (
	"io/fs"
	"time"
)

// fileInfo implements fs.FileInfo over a node. Its ModTime is the zero time.
type fileInfo struct {
	name string
	node *node
}

// Name returns the base name.
func (i fileInfo) Name() string { return i.name }

// Size returns the content length, zero for folders, and the target length for symbolic links.
func (i fileInfo) Size() int64 {
	if i.node.link != "" {
		return int64(len(i.node.link))
	}
	return int64(len(i.node.data))
}

// Mode returns the permission bits, with the directory bit for folders and the symlink bit for links.
func (i fileInfo) Mode() fs.FileMode {
	if i.node.link != "" {
		return fs.ModeSymlink | 0o777
	}
	if i.node.dir {
		return fs.ModeDir | i.node.mode.Perm()
	}
	return i.node.mode.Perm()
}

// ModTime returns the time set with SetModTime, or the zero time when none was set.
func (i fileInfo) ModTime() time.Time { return i.node.mtime }

// IsDir reports whether the node is a folder.
func (i fileInfo) IsDir() bool { return i.node.dir }

// Sys returns nil; the fake has no underlying system data.
func (i fileInfo) Sys() any { return nil }
