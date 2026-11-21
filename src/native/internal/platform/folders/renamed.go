package folders

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// AdoptRenamed moves the folder the app kept under an earlier name to current, once: only when current does not exist
// yet and previous does. It reports whether it moved the folder. Both folders sit in the same parent, so the move is a
// rename and never copies.
func AdoptRenamed(previous, current string) (bool, error) {
	if _, err := os.Lstat(current); !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if _, err := os.Lstat(previous); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(current), 0o755); err != nil {
		return false, err
	}
	if err := os.Rename(previous, current); err != nil {
		return false, err
	}
	return true, nil
}
