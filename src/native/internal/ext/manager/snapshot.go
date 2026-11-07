package manager

// SnapshotIdentity scopes an in-memory presentation snapshot to an enabled installation.
// The opaque identity is replaced on disable or installation replacement.
func (m *Manager) SnapshotIdentity(id string) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || e.builtin || e.runtime == nil || !m.isRunnable(e) {
		return nil
	}
	if e.snapshotIdentity == nil {
		e.snapshotIdentity = new(byte)
	}
	return e.snapshotIdentity
}
