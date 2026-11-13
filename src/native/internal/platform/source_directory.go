package platform

import "io/fs"

// SourceDirectory pins a directory while reading a bounded set of regular source files.
// Callers close it on all paths. It never permits traversal outside that directory.
type SourceDirectory interface {
	Entries(limit int) ([]fs.DirEntry, error)
	ReadFile(name string, max int64) ([]byte, error)
	Close() error
}

// SourceDirectories opens one child directory relative to a trusted source root.
type SourceDirectories interface {
	OpenSourceDirectory(root, name string) (SourceDirectory, error)
}
