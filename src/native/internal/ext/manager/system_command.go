package manager

import (
	"context"
	"errors"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
)

func (m *Manager) systemCommandSupported(op string) bool {
	id, known := systemfeat.CommandForOp(op)
	return known && m.serviceDeps.SystemCommands.Supported(id)
}

// SystemCommandAction binds confirmed dispatch to the actual enabled installation, never an author-supplied origin.
func (m *Manager) SystemCommandAction(ctx context.Context, id, op, pane string, identity any) error {
	m.mu.Lock()
	entry, err := m.findLocked(id)
	revision, _ := identity.(*byte)
	if err != nil || !m.isRunnable(entry) || entry.host == nil || (!entry.builtin && (entry.runtime == nil || revision == nil || revision != entry.snapshotIdentity)) {
		m.mu.Unlock()
		return errors.New("this extension action has expired; reopen its view")
	}
	host := entry.host
	m.mu.Unlock()
	return host.SystemCommandAction(ctx, op, pane)
}
