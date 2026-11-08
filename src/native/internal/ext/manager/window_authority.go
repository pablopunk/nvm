package manager

import (
	"sync/atomic"
)

// WindowAuthority identifies one admitted installation and is revoked on disable or replacement.
type WindowAuthority struct {
	identity any
	revoked  *atomic.Bool
}

func (a *WindowAuthority) Identity() any { return a.identity }
func (a *WindowAuthority) Valid() bool   { return a != nil && a.revoked != nil && !a.revoked.Load() }

// WindowAuthorityFor selects the origin under the manager lock, without retaining it across UI work.
func (m *Manager) WindowAuthorityFor(id string, identity any) (*WindowAuthority, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || e.runtime == nil || !m.isRunnable(e) {
		return nil, ErrChangedInstallation
	}
	var actual any
	if !e.builtin {
		if e.snapshotIdentity == nil {
			e.snapshotIdentity = new(byte)
		}
		actual = e.snapshotIdentity
	}
	if !sameInvocationIdentity(actual, identity) {
		return nil, ErrChangedInstallation
	}
	if e.revoked == nil {
		e.revoked = &atomic.Bool{}
	}
	return &WindowAuthority{identity: actual, revoked: e.revoked}, nil
}
