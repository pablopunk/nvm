package fake

import (
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
	out    chan platform.Event
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
		if !isUnder(event.Path, w.prefix) {
			continue
		}
		select {
		case w.out <- w.spell(event):
		default:
		}
	}
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
