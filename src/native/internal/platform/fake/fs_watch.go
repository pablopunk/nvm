package fake

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// watcherBuffer is how many events a watcher holds before further events are dropped.
const watcherBuffer = 64

// fsWatcher receives events for paths at or under prefix.
type fsWatcher struct {
	prefix string
	out    chan platform.Event
}

// notifyLocked delivers an event to every watcher whose folder contains its path. Sends never block: a full
// buffer drops the event. The caller holds f.mu.
func (f *FS) notifyLocked(event platform.Event) {
	for _, w := range f.watchers {
		if !isUnder(event.Path, w.prefix) {
			continue
		}
		select {
		case w.out <- event:
		default:
		}
	}
}

// isUnder reports whether key is root or lies beneath it. Both are cleaned tree keys.
func isUnder(key, root string) bool {
	if root == "/" {
		return true
	}
	return key == root || strings.HasPrefix(key, root+"/")
}
