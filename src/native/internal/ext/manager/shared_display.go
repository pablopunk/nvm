package manager

import "github.com/pablopunk/nvm/native/internal/ext/api"

// SharedDisplay returns the display text a built-in publishes for setting ref, for the settings row of reader. Only a
// reader whose manifest lists ref in Reads gets it, so a built-in reads another's settings only as it declares.
func (m *Manager) SharedDisplay(readerID string, ref api.SettingRef) (string, bool) {
	if !m.readsSharedSetting(readerID, ref) {
		return "", false
	}
	return m.serviceDeps.Displays.Read(ref)
}

func (m *Manager) readsSharedSetting(readerID string, ref api.SettingRef) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[builtinKeyPrefix+readerID]
	if !ok {
		return false
	}
	for _, declared := range entry.bundle.Manifest.Reads {
		if declared == ref {
			return true
		}
	}
	return false
}
