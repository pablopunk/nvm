package manager

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// WindowAction admits a declarative window effect for its originating installation.
func (m *Manager) WindowAction(id string, action api.Action, identity any, perform func(string, api.Action) error) error {
	m.mu.Lock()
	entry, err := m.findLocked(id)
	revision, _ := identity.(*byte)
	if err != nil || !m.isRunnable(entry) || entry.host == nil || (!entry.builtin && (entry.runtime == nil || revision == nil || revision != entry.snapshotIdentity)) {
		m.mu.Unlock()
		return errors.New("this extension action has expired; reopen its view")
	}
	m.mu.Unlock()
	return perform(id, action)
}
