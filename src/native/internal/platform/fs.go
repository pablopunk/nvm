package platform

import (
	"context"
	"io/fs"
)

// FS is the intent "read, change and watch files on this computer". Paths are native OS paths. Every OS
// supports it; watch granularity and event names differ per OS, so events are hints to read again.
type FS interface {
	// ReadFile returns the content of a file.
	ReadFile(name string) ([]byte, error)
	// WriteFile replaces the content of a file, creating it with perm when missing. Callers that need
	// crash safety write a temporary file and rename it; the port does not promise atomicity.
	WriteFile(name string, data []byte, perm fs.FileMode) error
	// CreateExclusive writes a file only when name does not exist yet, creating it with perm. A taken
	// name returns an error matching fs.ErrExist and changes nothing. It claims a name atomically, as a
	// single-writer lock needs.
	CreateExclusive(name string, data []byte, perm fs.FileMode) error
	// Sync flushes a file's content to stable storage. Some OSes refuse to sync folders; callers treat a
	// folder sync as best-effort.
	Sync(name string) error
	// Rename moves a file or folder, replacing a file at the destination.
	Rename(oldName, newName string) error
	// Remove deletes a file or an empty folder. It never moves to the Trash; see Shell.Trash.
	Remove(name string) error
	// MkdirAll creates a folder and its parents.
	MkdirAll(name string, perm fs.FileMode) error
	// Stat describes a file or folder, following a symbolic link to its target.
	Stat(name string) (fs.FileInfo, error)
	// Lstat describes a file or folder without following a symbolic link: a link describes itself.
	Lstat(name string) (fs.FileInfo, error)
	// ReadFilePrefix returns at most max leading bytes of a file together with the file's full size. It
	// never loads the rest of a large file, so previews stay bounded.
	ReadFilePrefix(name string, max int64) (data []byte, size int64, err error)
	// ReadDir lists a folder, sorted by name.
	ReadDir(name string) ([]fs.DirEntry, error)
	// Walk visits a tree in lexical order, as filepath.WalkDir does.
	Walk(root string, fn fs.WalkDirFunc) error
	// Chmod changes the permission bits of a file or folder.
	Chmod(name string, mode fs.FileMode) error
	// Watch reports changes under name until ctx is done, then closes the channel. Events are hints:
	// read the files again. Events may be dropped when a consumer falls behind.
	Watch(ctx context.Context, name string) (<-chan Event, error)
}

// EventOp is what happened to a watched path.
type EventOp string

// Event kinds reported by FS.Watch.
const (
	EventCreate EventOp = "create"
	EventWrite  EventOp = "write"
	EventRemove EventOp = "remove"
	EventRename EventOp = "rename"
)

// Event is one change under a watched folder. OldPath is set for EventRename only.
type Event struct {
	Op      EventOp
	Path    string
	OldPath string
	IsDir   bool
}
