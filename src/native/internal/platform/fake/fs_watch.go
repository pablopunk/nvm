package fake

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// watcherBuffer is how many events a watcher holds before further events are dropped.
const watcherBuffer = 64

// fsWatcher receives events for paths at or under prefix. root is the watched name in the spelling the caller opened
// it with, and events are reported under it, as a real watcher reports paths under the folder it was given.
type fsWatcher struct {
	prefix string
	root   string
	// skip drops the entries and folders WatchFiltered was asked to leave out; nil keeps everything.
	skip func(rel string, isDir bool) bool
	out  chan platform.Event
	// closed is set once out is closed, by BreakWatches or by the watch's context ending; the other one skips it.
	closed bool
}

// BreakWatches simulates the OS dropping watches under name: their channels close while their contexts are still
// live, which is how a lost watch looks to the code under test. The watches are detached, as if the OS had forgotten
// them, so later changes are not reported to them.
func (f *FS) BreakWatches(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	kept := f.watchers[:0]
	for _, w := range f.watchers {
		if !isUnder(key, w.prefix) && !isUnder(w.prefix, key) {
			kept = append(kept, w)
			continue
		}
		if !w.closed {
			w.closed = true
			close(w.out)
		}
	}
	f.watchers = kept
}

// notifyLocked delivers an event to every watcher whose folder contains its path. Sends never block: a full
// buffer drops the event. The caller holds f.mu.
func (f *FS) notifyLocked(event platform.Event) {
	for _, w := range f.watchers {
		if !isUnder(event.Path, w.prefix) || w.skipped(event.Path, event.IsDir) {
			continue
		}
		select {
		case w.out <- w.spell(event):
		default:
		}
	}
}

// Watch reports changes at or under name until ctx is done, then closes the channel. Events that do not fit the
// buffer are dropped, as the port allows.
func (f *FS) Watch(ctx context.Context, name string) (<-chan platform.Event, error) {
	return f.WatchFiltered(ctx, name, nil)
}

// WatchFiltered is Watch without the entries skip returns true for. A watch whose entries exceed the limit set by
// SetWatchEntryLimit fails with platform.ErrWatchLimit, as the real port does.
func (f *FS) WatchFiltered(ctx context.Context, name string, skip func(rel string, isDir bool) bool) (<-chan platform.Event, error) {
	if err := f.faults.take(OpWatch); err != nil {
		return nil, pathError("watch", name, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	if _, err := f.lookupLocked(key); err != nil {
		return nil, pathError("watch", name, err)
	}
	if f.watchEntryLimit > 0 && f.watchEntriesLocked(key, skip) > f.watchEntryLimit {
		return nil, pathError("watch", name, platform.ErrWatchLimit)
	}
	w := &fsWatcher{prefix: key, root: watchSpelling(name, key), skip: skip, out: make(chan platform.Event, watcherBuffer)}
	f.watchers = append(f.watchers, w)
	go f.unwatchOnDone(ctx, w)
	return w.out, nil
}

// SetWatchEntryLimit makes a watch fail with platform.ErrWatchLimit when more entries than limit lie under its folder
// after its skip filter. Zero removes the limit.
func (f *FS) SetWatchEntryLimit(limit int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.watchEntryLimit = limit
}

// watchEntriesLocked counts the entries a watch of key would enroll: every file and folder beneath key that skip keeps,
// and nothing beneath a folder it drops. The caller holds f.mu.
func (f *FS) watchEntriesLocked(key string, skip func(rel string, isDir bool) bool) int {
	count := 0
	folders := []string{key}
	for len(folders) > 0 {
		dir := folders[len(folders)-1]
		folders = folders[:len(folders)-1]
		for _, child := range f.childrenLocked(dir) {
			isDir := f.nodes[child].dir
			if skip != nil && skip(keyRelativeTo(key, child), isDir) {
				continue
			}
			count++
			if isDir {
				folders = append(folders, child)
			}
		}
	}
	return count
}

// keyRelativeTo returns the slash-separated path of key under root, for a key beneath root.
func keyRelativeTo(root, key string) string {
	return strings.TrimPrefix(key, strings.TrimSuffix(root, "/")+"/")
}

// skipped reports whether an event at path is dropped by the watch's skip filter: the entry itself, or a folder above
// it on the way from the watched folder.
func (w *fsWatcher) skipped(path string, isDir bool) bool {
	if w.skip == nil || path == w.prefix {
		return false
	}
	parts := strings.Split(keyRelativeTo(w.prefix, path), "/")
	for i := 1; i < len(parts); i++ {
		if w.skip(strings.Join(parts[:i], "/"), true) {
			return true
		}
	}
	return w.skip(strings.Join(parts, "/"), isDir)
}

// watchSpelling is the folder name a watch reports under: the caller's name when it is absolute on this OS, and the
// tree key otherwise.
func watchSpelling(name, key string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.FromSlash(key)
}

// spell rewrites an event's paths from tree keys into the spelling of the watched root. A path outside the watch,
// such as the source of a rename from elsewhere, keeps its key.
func (w *fsWatcher) spell(event platform.Event) platform.Event {
	event.Path = w.spellKey(event.Path)
	if event.OldPath != "" && isUnder(event.OldPath, w.prefix) {
		event.OldPath = w.spellKey(event.OldPath)
	}
	return event
}

func (w *fsWatcher) spellKey(key string) string {
	below := strings.TrimPrefix(strings.TrimPrefix(key, w.prefix), "/")
	return filepath.Join(w.root, filepath.FromSlash(below))
}

// isUnder reports whether key is root or lies beneath it. Both are cleaned tree keys.
func isUnder(key, root string) bool {
	if root == "/" {
		return true
	}
	return key == root || strings.HasPrefix(key, root+"/")
}

// OverflowWatches simulates the OS reporting that the watches under name lost changes: each one first receives an
// EventOverflow naming its folder, then its channel closes while its context is still live, as the real port does
// when a watch hits its entry limit.
func (f *FS) OverflowWatches(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	kept := f.watchers[:0]
	for _, w := range f.watchers {
		if !isUnder(key, w.prefix) && !isUnder(w.prefix, key) {
			kept = append(kept, w)
			continue
		}
		if !w.closed {
			w.closed = true
			select {
			case w.out <- platform.Event{Op: platform.EventOverflow, Path: w.root}:
			default:
			}
			close(w.out)
		}
	}
	f.watchers = kept
}
