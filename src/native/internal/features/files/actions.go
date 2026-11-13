package files

import (
	"errors"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrNotAFile is returned by the desktop actions for paths that could name more than one item, such as a file system
// root or the home folder itself. Trash refuses them outright.
var ErrNotAFile = errors.New("this path names a folder that cannot be changed from here")

// OpenWithApps lists the applications that can open a file, most suitable first. The Shell port does not offer this
// yet: the operating system sources (Launch Services, desktop entries, file associations) arrive with WP1.7b.
type OpenWithApps interface {
	AppsFor(path string) ([]platform.App, error)
}

// Reveal selects path in the system file manager. Only absolute paths are accepted: a relative one could name a
// different item than the user chose.
func Reveal(shell platform.Shell, path string) error {
	if !filepath.IsAbs(path) {
		return errNotAbsolute
	}
	return shell.Reveal(path)
}

// QuickLook shows path in the system's quick preview. Only absolute paths are accepted.
func QuickLook(shell platform.Shell, path string) error {
	if !filepath.IsAbs(path) {
		return errNotAbsolute
	}
	return shell.QuickLook(path)
}

// OpenPath opens path with its default application. Only absolute paths are accepted.
func OpenPath(shell platform.Shell, path string) error {
	if !filepath.IsAbs(path) {
		return errNotAbsolute
	}
	return shell.OpenPath(path)
}

// Trash moves path to the Trash. It refuses file system roots and the home folder, which a stray selection could
// otherwise name, and it refuses relative paths. The shell moves items to the Trash and never deletes them.
func Trash(shell platform.Shell, home, path string) error {
	if !filepath.IsAbs(path) {
		return errNotAbsolute
	}
	clean := filepath.Clean(path)
	if clean == filepath.Dir(clean) || (home != "" && clean == filepath.Clean(home)) {
		return ErrNotAFile
	}
	return shell.Trash(clean)
}
