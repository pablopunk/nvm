package geometry

import (
	"strconv"
	"strings"
)

// MaxRememberedWindows caps the history store; the oldest window is forgotten first.
const MaxRememberedWindows = 200

// AppliedLayout is the last layout a window received through Nevermind.
type AppliedLayout struct {
	Frame      Rect
	CycleGroup Edge
	CycleIndex int
}

// History is what the engine remembers about one window: the frame to restore to and the last
// applied layout, which decides whether a repeated command continues a cycle.
type History struct {
	RestoreFrame *Rect
	LastApplied  *AppliedLayout
}

// movedSinceLastApplied reports whether the window is no longer where the last layout put it.
func movedSinceLastApplied(previous *History, current Rect) bool {
	return !(previous.LastApplied != nil && FramesMatch(previous.LastApplied.Frame, current, CycleTolerancePoints))
}

// shouldCaptureRestoreFrame decides whether the pre-layout frame becomes the restore target. It is
// captured on the first change, and again when the user has moved the window since the last layout.
func shouldCaptureRestoreFrame(previous *History, current Rect, changed bool) bool {
	return changed && (previous == nil || previous.RestoreFrame == nil || movedSinceLastApplied(previous, current))
}

// RememberApplied records a successful layout. window is the frame before the change, applied the
// frame read back afterwards, changed whether the frame actually moved, and cycle the cycle state.
func RememberApplied(previous *History, window WindowFrame, applied Rect, changed bool, cycle CycleRecord) History {
	next := History{LastApplied: &AppliedLayout{Frame: applied, CycleGroup: cycle.Group, CycleIndex: cycle.Index}}
	switch {
	case shouldCaptureRestoreFrame(previous, window.Frame, changed):
		frame := window.Frame
		next.RestoreFrame = &frame
	case previous != nil:
		next.RestoreFrame = previous.RestoreFrame
	}
	return next
}

// ForgetCycleGroup keeps the restore frame and the last applied frame but ends the cycle, so the
// next half command starts again at the half. Full-screen toggles use this.
func ForgetCycleGroup(previous History) History {
	if previous.LastApplied == nil {
		return previous
	}
	last := *previous.LastApplied
	last.CycleGroup = ""
	return History{RestoreFrame: previous.RestoreFrame, LastApplied: &last}
}

// Memory is the keyed store of per-window histories. It keeps insertion order so the oldest entry
// is evicted at the cap. It is not safe for concurrent use; the engine owns it on one goroutine.
type Memory struct {
	order   []string
	entries map[string]History
}

// NewMemory returns an empty store.
func NewMemory() *Memory {
	return &Memory{entries: map[string]History{}}
}

// Read returns the history for a window key.
func (m *Memory) Read(windowKey string) (History, bool) {
	entry, ok := m.entries[windowKey]
	return entry, ok
}

// Write stores a history and makes it the newest entry, evicting the oldest past the cap.
func (m *Memory) Write(windowKey string, entry History) {
	m.Forget(windowKey)
	m.entries[windowKey] = entry
	m.order = append(m.order, windowKey)
	if len(m.order) > MaxRememberedWindows {
		oldest := m.order[0]
		m.order = m.order[1:]
		delete(m.entries, oldest)
	}
}

// Forget removes a window's history.
func (m *Memory) Forget(windowKey string) {
	if _, ok := m.entries[windowKey]; !ok {
		return
	}
	delete(m.entries, windowKey)
	for index, key := range m.order {
		if key == windowKey {
			m.order = append(m.order[:index], m.order[index+1:]...)
			return
		}
	}
}

// DropProcess forgets every window of one process. Keys have the form "<pid>:<window>".
func (m *Memory) DropProcess(pid int) {
	prefix := strconv.Itoa(pid) + ":"
	for _, key := range append([]string(nil), m.order...) {
		if strings.HasPrefix(key, prefix) {
			m.Forget(key)
		}
	}
}

// Len returns the number of remembered windows.
func (m *Memory) Len() int {
	return len(m.entries)
}
