package real

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// osFS is the operating system's file system.
type osFS struct{}

// ReadFile returns the content of a file.
func (osFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

// WriteFile writes a file, creating it with perm when missing.
func (osFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// Rename moves a file or folder.
func (osFS) Rename(oldName, newName string) error { return os.Rename(oldName, newName) }

// Remove deletes a file or an empty folder.
func (osFS) Remove(name string) error { return os.Remove(name) }

// MkdirAll creates a folder and its parents.
func (osFS) MkdirAll(name string, perm fs.FileMode) error { return os.MkdirAll(name, perm) }

// Stat describes a file or folder.
func (osFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

// Lstat describes a file or folder without following a symbolic link.
func (osFS) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(name) }

// ReadDir lists a folder sorted by name.
func (osFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }

// Walk visits a tree in lexical order.
func (osFS) Walk(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }

// Chmod changes the permission bits. On Windows only the read-only bit has an effect.
func (osFS) Chmod(name string, mode fs.FileMode) error { return os.Chmod(name, mode) }

// Watch reports changes under name until ctx is done. See fs_watch.go for how events map.
func (osFS) Watch(ctx context.Context, name string) (<-chan platform.Event, error) {
	return startWatch(ctx, name)
}
