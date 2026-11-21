package real

import (
	"errors"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io"
	"io/fs"
	"os"
	"strings"
)

type sourceDirectory struct{ root *os.Root }

func (osFS) OpenSourceDirectory(root, name string) (platform.SourceDirectory, error) {
	if !sourceBaseName(name) {
		return nil, fs.ErrInvalid
	}
	parent, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&fs.ModeSymlink != 0 {
		return nil, platform.ErrFileSymlink
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		_ = child.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("source directory changed while opening")
	}
	return &sourceDirectory{root: child}, nil
}

func sourceBaseName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:`) && !strings.ContainsRune(name, 0)
}

func (d *sourceDirectory) Close() error { return d.root.Close() }
func (d *sourceDirectory) Entries(limit int) ([]fs.DirEntry, error) {
	if limit < 0 || limit > 1024 {
		return nil, fs.ErrInvalid
	}
	file, err := d.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > limit {
		return nil, fmt.Errorf("source directory exceeds %d entries", limit)
	}
	return entries, nil
}
func (d *sourceDirectory) ReadFile(name string, max int64) ([]byte, error) {
	if !sourceBaseName(name) || max < 0 || max > 64<<20 {
		return nil, fs.ErrInvalid
	}
	before, err := d.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, platform.ErrFileSymlink
	}
	file, err := openSnapshotFile(d.root, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, errors.New("source file changed while opening")
	}
	if opened.Size() > max {
		return nil, platform.ErrFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	after, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) || int64(len(data)) != after.Size() {
		return nil, errors.New("source file changed while reading")
	}
	return data, nil
}
