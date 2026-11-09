package manager

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// ActionSupported is a cheap platform snapshot. It is discovery information, never an authorization grant.
func (m *Manager) ActionSupported(action api.Action) bool {
	if action.Kind == api.ActionQuickLook {
		return platform.BindFilePreview(m.serviceDeps.Shell, m.serviceDeps.Capabilities).Supported()
	}
	return true
}

// PreviewFileAction runs for the installation that produced the declarative action, including after confirmation.
func (m *Manager) PreviewFileAction(ctx context.Context, id, path string, identity any) error {
	m.mu.Lock()
	e, err := m.findLocked(id)
	revision, _ := identity.(*byte)
	if err != nil || !m.isRunnable(e) || e.host == nil || (!e.builtin && (e.runtime == nil || revision == nil || revision != e.snapshotIdentity)) {
		m.mu.Unlock()
		return errors.New("this extension action has expired; reopen its view")
	}
	host := e.host
	m.mu.Unlock()
	return host.PreviewFileAction(ctx, path)
}
