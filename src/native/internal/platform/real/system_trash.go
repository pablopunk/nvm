package real

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// emptyFolder removes every entry of a folder and keeps the folder itself. A missing folder is already empty.
func emptyFolder(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
