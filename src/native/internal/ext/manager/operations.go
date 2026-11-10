package manager

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// OperationsSupported reads only the bindings wired at startup. It never probes a resource or grants access.
func (m *Manager) OperationsSupported(required []api.Operation) bool {
	for _, operation := range required {
		switch operation {
		case api.OperationFilePreview:
			if !platform.BindFilePreview(m.serviceDeps.Shell, m.serviceDeps.Capabilities).Supported() {
				return false
			}
		case api.OperationSwitchDesktop:
			if m.serviceDeps.Windows == nil || m.serviceDeps.Desktops == nil || !m.serviceDeps.Desktops.Supported() {
				return false
			}
		default:
			if !m.serviceDeps.SystemCommands.SupportsCapability(platform.Capability(operation)) {
				return false
			}
		}
	}
	return true
}

func (m *Manager) commandSupported(id, commandID string) bool {
	result, known := m.commands.lookup(id)
	if !known {
		return true
	}
	for _, command := range result.Commands {
		if command.ID == commandID {
			return m.OperationsSupported(command.Requires)
		}
	}
	return true
}
