package files

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// errNotAbsolute is returned for paths that do not name one file unambiguously.
var errNotAbsolute = errors.New("writeText needs an absolute path or one starting with ~")

// WriteOptions controls WriteTextFile.
type WriteOptions struct {
	// Overwrite replaces an existing file. Without it an existing file is an error and nothing changes.
	Overwrite bool
}

// WriteTextFile writes UTF-8 text to an absolute path, or one starting with "~" that home expands. The folder must
// already exist: the call never creates folders. It returns the path written.
func WriteTextFile(fsys platform.FS, home, path, text string, opts WriteOptions) (string, error) {
	if path == "" {
		return "", errNotAbsolute
	}
	target := ExpandUserPath(home, path)
	if !filepath.IsAbs(target) {
		return "", errNotAbsolute
	}
	folder := filepath.Dir(target)
	if info, err := fsys.Stat(folder); err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder %s does not exist", DisplayPath(home, folder))
	}
	if opts.Overwrite {
		if err := replaceFile(fsys, target, []byte(text)); err != nil {
			return "", err
		}
		return target, nil
	}
	if err := fsys.CreateExclusive(target, []byte(text), 0o644); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("%s already exists; pass Overwrite to replace it", DisplayPath(home, target))
		}
		return "", err
	}
	return target, nil
}

// replaceFile writes data to a sibling temporary file, flushes it and renames it over target.
func replaceFile(fsys platform.FS, target string, data []byte) error {
	tmp := target + ".nvm-tmp"
	if err := fsys.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := fsys.Sync(tmp); err != nil {
		return err
	}
	return fsys.Rename(tmp, target)
}
