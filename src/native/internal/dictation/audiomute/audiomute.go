// Package audiomute mutes the speakers while a dictation records and gives back the state they were in: when the
// dictation stops, is cancelled or fails. The state it replaces is written to a journal first, so a crash mid-dictation
// is undone at the next start, as the Electron app does.
package audiomute

import (
	"errors"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Journal holds the output state a mute replaced until it is given back.
type Journal interface {
	Save(wasMuted bool) error
	Load() (wasMuted bool, found bool, err error)
	Clear() error
}

// Muter mutes an output through the platform port and journals what it replaces.
type Muter struct {
	output  platform.OutputMuter
	journal Journal
}

// New returns a muter over output that keeps its journal in journal.
func New(output platform.OutputMuter, journal Journal) *Muter {
	return &Muter{output: output, journal: journal}
}

// Lease is one mute. Restore gives back the state it replaced; a nil lease is a mute that did not happen.
type Lease struct {
	muter    *Muter
	wasMuted bool
	once     sync.Once
	err      error
}

// Begin mutes the output and journals the state it replaces. An OS that cannot mute gets a nil lease and no error, so
// the dictation goes on unmuted. A journal that cannot be written stops the mute before it happens.
func (m *Muter) Begin() (*Lease, error) {
	wasMuted, err := m.output.Muted()
	if errors.Is(err, platform.ErrUnsupported) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := m.journal.Save(wasMuted); err != nil {
		return nil, err
	}
	if err := m.output.SetMuted(true); err != nil {
		_ = m.journal.Clear()
		return nil, err
	}
	return &Lease{muter: m, wasMuted: wasMuted}, nil
}

// Restore gives back the state the mute replaced and clears the journal. It is safe to call more than once and on a
// nil lease. A failure keeps the journal, so the next start gives the state back.
func (l *Lease) Restore() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		if err := l.muter.output.SetMuted(l.wasMuted); err != nil {
			l.err = err
			return
		}
		l.err = l.muter.journal.Clear()
	})
	return l.err
}

// RecoverAfterCrash gives back the state of a mute that never restored. The journal holds an entry only while a
// dictation's mute is in effect, so an entry at start means the app stopped in the middle of one.
func (m *Muter) RecoverAfterCrash() error {
	wasMuted, found, err := m.journal.Load()
	if err != nil || !found {
		return err
	}
	if err := m.output.SetMuted(wasMuted); err != nil {
		return err
	}
	return m.journal.Clear()
}
