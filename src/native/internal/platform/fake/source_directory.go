package fake

import (
	"errors"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"
)

type sourceDirectory struct {
	files  *FS
	path   string
	node   *node
	closed atomic.Bool
}

func sourceBaseName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:`) && !strings.ContainsRune(name, 0)
}
func (f *FS) OpenSourceDirectory(root, name string) (platform.SourceDirectory, error) {
	if !sourceBaseName(name) {
		return nil, fs.ErrInvalid
	}
	path := filepath.Join(root, name)
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nodes[cleanKey(path)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if !n.dir || n.link != "" {
		return nil, platform.ErrFileSymlink
	}
	return &sourceDirectory{files: f, path: path, node: n}, nil
}
func (d *sourceDirectory) current() bool {
	d.files.mu.Lock()
	defer d.files.mu.Unlock()
	return !d.closed.Load() && d.files.nodes[cleanKey(d.path)] == d.node
}
func (d *sourceDirectory) Close() error { d.closed.Store(true); return nil }
func (d *sourceDirectory) Entries(limit int) ([]fs.DirEntry, error) {
	if !d.current() {
		return nil, fs.ErrNotExist
	}
	entries, err := d.files.ReadDir(d.path)
	if err != nil {
		return nil, err
	}
	if limit < 0 || len(entries) > limit {
		return nil, fs.ErrInvalid
	}
	return entries, nil
}
func (d *sourceDirectory) ReadFile(name string, max int64) ([]byte, error) {
	if !sourceBaseName(name) || max < 0 {
		return nil, fs.ErrInvalid
	}
	if !d.current() {
		return nil, fs.ErrNotExist
	}
	source, err := d.files.OpenReadNoFollow(filepath.Join(d.path, name))
	if err != nil {
		return nil, err
	}
	defer source.Close()
	if !d.current() {
		return nil, errors.New("source directory changed")
	}
	data, err := io.ReadAll(io.LimitReader(source, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	return data, nil
}
