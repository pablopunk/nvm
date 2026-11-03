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

// Size returns the content length, zero for folders.
func (i fileInfo) Size() int64 { return int64(len(i.node.data)) }

// Mode returns the permission bits, with the directory bit for folders.
func (i fileInfo) Mode() fs.FileMode {
	if i.node.dir {
		return fs.ModeDir | i.node.mode.Perm()
	}
	return i.node.mode.Perm()
}

// ModTime returns the zero time; the fake does not model modification times.
func (i fileInfo) ModTime() time.Time { return time.Time{} }

// IsDir reports whether the node is a folder.
func (i fileInfo) IsDir() bool { return i.node.dir }

// Sys returns nil; the fake has no underlying system data.
func (i fileInfo) Sys() any { return nil }
