package manager

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/services"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// WindowAction admits a declarative window effect for its originating installation.
func (m *Manager) WindowAction(ctx context.Context, id string, action api.Action, identity any, windows services.ViewWindowActions) error {
	m.mu.Lock()
	entry, err := m.findLocked(id)
	revision, _ := identity.(*byte)
	if err != nil || !m.isRunnable(entry) || entry.host == nil || (!entry.builtin && (entry.runtime == nil || revision == nil || revision != entry.snapshotIdentity)) {
		m.mu.Unlock()
		return errors.New("this extension action has expired; reopen its view")
	}
	host := entry.host
	m.mu.Unlock()
	return host.WindowAction(ctx, action, windows)
}
