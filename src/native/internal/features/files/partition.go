package files

import (
	"errors"
	"io/fs"
	"syscall"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// PartitionRoots splits roots into those that exist and those that are missing, keeping the caller's order. A
// root is missing when the OS says it does not exist or that a parent is not a folder. Any other error keeps the
// root in the existing list: the walk then reports it as unreadable instead of silently dropping it.
func PartitionRoots(fsys platform.FS, roots []string) (existing, missing []string) {
	for _, root := range roots {
		if _, err := fsys.Stat(root); err != nil && isMissingPath(err) {
			missing = append(missing, root)
			continue
		}
		existing = append(existing, root)
	}
	return existing, missing
}

// isMissingPath reports whether err means the path is absent rather than unreadable.
func isMissingPath(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
