package shortcuts

import (
	"context"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	systemReadTimeout = 5 * time.Second
	// systemFreshFor is how long a read of the system shortcuts answers recordings; the user may turn one on or off in
	// the system settings while Nevermind runs, so an older answer is read again in the background.
	systemFreshFor = 10 * time.Second
)

// systemOwners knows which shortcuts the OS has turned on right now. It answers from the last read and reads again off
// the caller's thread, so a recording never waits for the OS. Before the first read finishes, and after a failed one,
// it answers that no shortcut is taken.
type systemOwners struct {
	source platform.SystemShortcuts
	clock  platform.Clock

	mu      sync.Mutex
	taken   map[string]string
	readAt  time.Time
	reading bool
}

func newSystemOwners(source platform.SystemShortcuts, clock platform.Clock) *systemOwners {
	return &systemOwners{source: source, clock: clock}
}

// Refresh reads the system shortcuts in the background, unless a read is already running.
func (s *systemOwners) Refresh() {
	if s == nil || s.source == nil {
		return
	}
	s.mu.Lock()
	if s.reading {
		s.mu.Unlock()
		return
	}
	s.reading = true
	s.mu.Unlock()
	go s.read()
}

// read is the blocking read. A failure keeps the previous answer, or none, so a broken read never blocks a shortcut.
func (s *systemOwners) read() {
	ctx, cancel := context.WithTimeout(context.Background(), systemReadTimeout)
	defer cancel()
	list, err := s.source.Enabled(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reading = false
	if err != nil {
		return
	}
	taken := make(map[string]string, len(list))
	for _, shortcut := range list {
		taken[systemAccelerator(shortcut)] = shortcut.Name
	}
	s.taken, s.readAt = taken, s.clock.Now()
}

// Owner names the system feature that has accelerator turned on, or "" when none does.
func (s *systemOwners) Owner(accelerator string) string {
	if s == nil || s.source == nil {
		return ""
	}
	s.mu.Lock()
	name := s.taken[inCanonicalOrder(accelerator)]
	stale := s.readAt.IsZero() || s.clock.Now().Sub(s.readAt) > systemFreshFor
	s.mu.Unlock()
	if stale {
		s.Refresh()
	}
	return name
}

func systemAccelerator(shortcut platform.SystemShortcut) string {
	mods := Modifiers{Command: shortcut.Command, Control: shortcut.Control, Alt: shortcut.Alt, Shift: shortcut.Shift}
	return Capture(mods, shortcut.Key)
}

// inCanonicalOrder spells accelerator with its modifiers in the order the recorder stores them, so Shift+Command+4 and
// Command+Shift+4 are one shortcut.
func inCanonicalOrder(accelerator string) string {
	parts := Parts(accelerator)
	if len(parts) < 2 {
		return Normalize(accelerator)
	}
	var mods Modifiers
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case Command:
			mods.Command = true
		case Control:
			mods.Control = true
		case Alt:
			mods.Alt = true
		case Shift:
			mods.Shift = true
		default:
			return Normalize(accelerator)
		}
	}
	return Capture(mods, parts[len(parts)-1])
}
