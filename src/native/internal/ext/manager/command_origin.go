package manager

// CommandOrigin is an immutable, opaque snapshot of a command's registered source.
type CommandOrigin struct {
	extensionID string
	commandID   string
}

// ExtensionID returns the validated built-in extension ID, or empty for non-built-in sources.
func (o CommandOrigin) ExtensionID() string { return o.extensionID }

// CommandID returns the validated cached command ID, or empty when unavailable.
func (o CommandOrigin) CommandID() string { return o.commandID }

// ResolveCommandOrigin classifies only the current registry and known command snapshot; it never discovers commands.
func (m *Manager) ResolveCommandOrigin(id, commandID string) CommandOrigin {
	if m == nil {
		return CommandOrigin{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || !e.builtin {
		return CommandOrigin{}
	}
	extensionID := e.bundle.Manifest.ID
	if !validOriginID(extensionID) {
		return CommandOrigin{}
	}
	origin := CommandOrigin{extensionID: extensionID}
	if !validOriginID(commandID) {
		return origin
	}
	if commands, ok := m.commands.lookup(extensionID); ok {
		for _, command := range commands.Commands {
			if command.ID == commandID {
				origin.commandID = commandID
				break
			}
		}
	}
	return origin
}

func validOriginID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		alphaNum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alphaNum && (i == 0 || c != '.' && c != '_' && c != ':' && c != '-') {
			return false
		}
	}
	return true
}
