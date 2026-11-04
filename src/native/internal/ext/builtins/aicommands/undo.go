package aicommands

import (
	"sync"
	"time"
)

// undoWindow is how long after a fix the undo stays available. A later edit by the user makes an undo unsafe, so the
// window is short.
const undoWindow = 10 * time.Minute

// fixLog is the record of the last correction: when it was made, and whether it is still undoable.
type fixLog struct {
	mu     sync.Mutex
	at     time.Time
	active bool
}

// record notes that a correction was just made at now.
func (l *fixLog) record(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.at = now
	l.active = true
}

// pending reports whether the last correction can still be undone at now.
func (l *fixLog) pending(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active && now.Sub(l.at) <= undoWindow
}

// take reports whether the last correction can be undone at now, and forgets it, so one fix is undone at most once.
func (l *fixLog) take(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	undoable := l.active && now.Sub(l.at) <= undoWindow
	l.active = false
	return undoable
}
