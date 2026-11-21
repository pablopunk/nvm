package manager

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

func (m *Manager) shellActionSupported(command *api.ShellCommand) bool {
	if command == nil {
		return false
	}
	support := platform.ProcessAvailability(m.serviceDeps.Commands, m.serviceDeps.OS)
	if command.Script != "" {
		return support.LoginScript
	}
	return command.Command != "" && support.Exec
}

func (m *Manager) ShellAction(ctx context.Context, id string, command api.ShellCommand, identity any) (api.ShellResult, error) {
	m.mu.Lock()
	entry, err := m.findLocked(id)
	revision, _ := identity.(*byte)
	if err != nil || !m.isRunnable(entry) || entry.host == nil || (!entry.builtin && (entry.runtime == nil || revision == nil || revision != entry.snapshotIdentity)) {
		m.mu.Unlock()
		return api.ShellResult{}, errors.New("this extension action has expired; reopen its view")
	}
	host := entry.host
	m.mu.Unlock()
	return host.ShellAction(ctx, command)
}
