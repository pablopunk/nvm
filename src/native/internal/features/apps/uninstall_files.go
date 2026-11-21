package apps

import (
	"errors"
	"io/fs"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Files is the file-system view the uninstall flow reads through. Lstat never follows a link, so a link describes
// itself. The production implementation wraps the FS port; tests use an in-memory tree.
type Files interface {
	// Lstat describes name without following a symbolic link.
	Lstat(name string) (Entry, error)
	// ReadDir lists a folder's entries, sorted by name.
	ReadDir(name string) ([]DirEntry, error)
	// Tree counts name and everything under it. It stops once more than limit entries were seen and returns
	// errTreeTooLarge, so a huge folder costs bounded work.
	Tree(name string, limit int) (int, error)
	// ReadFile returns the content of a file.
	ReadFile(name string) ([]byte, error)
}

// Entry is what Lstat reports about one item.
type Entry struct {
	Dir      bool
	Symlink  bool
	Mode     fs.FileMode
	Identity Identity
}

// DirEntry is one name in a folder listing.
type DirEntry struct {
	Name    string
	Dir     bool
	Symlink bool
}

var errTreeTooLarge = errors.New("apps: tree exceeds the item limit")

// IdentityFunc reads the device, inode and owner of an item from its file info. It returns ok=false when the
// platform does not expose them; the guards then refuse the item, because ownership cannot be checked.
type IdentityFunc func(info fs.FileInfo) (dev, ino uint64, uid int, ok bool)

// PortFiles adapts the FS port to Files. Identify defaults to the operating system's stat fields.
type PortFiles struct {
	FS       platform.FS
	Identify IdentityFunc
}

// Lstat describes the item, reading its identity through Identify. Missing identity means owner -1, which no user
// owns, so an unknown owner is refused.
func (p PortFiles) Lstat(name string) (Entry, error) {
	info, err := p.FS.Lstat(name)
	if err != nil {
		return Entry{}, err
	}
	identify := p.Identify
	if identify == nil {
		identify = statIdentity
	}
	dev, ino, uid, ok := identify(info)
	if !ok {
		uid = -1
	}
	mode := info.Mode()
	return Entry{
		Dir:      info.IsDir(),
		Symlink:  mode&fs.ModeSymlink != 0,
		Mode:     mode,
		Identity: Identity{Dir: info.IsDir(), Dev: dev, Ino: ino, UID: uid},
	}, nil
}

// ReadDir lists the folder through the FS port.
func (p PortFiles) ReadDir(name string) ([]DirEntry, error) {
	entries, err := p.FS.ReadDir(name)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, DirEntry{Name: entry.Name(), Dir: entry.IsDir(), Symlink: entry.Type()&fs.ModeSymlink != 0})
	}
	return out, nil
}

// Tree counts the entries under name, walking without following links.
func (p PortFiles) Tree(name string, limit int) (int, error) {
	count := 0
	err := p.FS.Walk(name, func(string, fs.DirEntry, error) error {
		count++
		if count > limit {
			return errTreeTooLarge
		}
		return nil
	})
	if errors.Is(err, errTreeTooLarge) {
		return count, errTreeTooLarge
	}
	return count, err
}

// ReadFile reads the file through the FS port.
func (p PortFiles) ReadFile(name string) ([]byte, error) {
	return p.FS.ReadFile(name)
}

// isMissing reports whether an error means "no such item".
func isMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, platform.ErrNotFound)
}

// sameIdentity compares what makes an item the same file: its type, device and inode. The owner is checked
// separately, as the 1.x code does.
func sameIdentity(a, b Identity) bool {
	return a.Dir == b.Dir && a.Dev == b.Dev && a.Ino == b.Ino
}
