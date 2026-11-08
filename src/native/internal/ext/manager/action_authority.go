package manager

import (
	"sync/atomic"
)

// ActionAuthority identifies one admitted installation and is revoked on disable or replacement.
type ActionAuthority struct {
	identity any
	revoked  *atomic.Bool
}

func (a *ActionAuthority) Identity() any { return a.identity }
func (a *ActionAuthority) Valid() bool   { return a != nil && a.revoked != nil && !a.revoked.Load() }

// ActionAuthorityFor selects the origin under the manager lock, without retaining it across UI work.
func (m *Manager) ActionAuthorityFor(id string, identity any) (*ActionAuthority, error) {
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
	return &ActionAuthority{identity: actual, revoked: e.revoked}, nil
}
