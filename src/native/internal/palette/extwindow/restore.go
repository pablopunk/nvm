package extwindow

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Restorer asks the extension that owns a persistent window for its fresh view and options. It returns nil when the
// extension no longer restores the key, and an error when the call fails.
type Restorer func(extID, restoreKey string) (*api.WindowRestore, error)

// RestorePersisted reopens every saved persistent window. Each restore runs off the UI thread, as an extension call
// does; the window opens on the UI thread once its view comes back. A window the extension does not restore, or whose
// restore fails, is dropped from the saved state, as the 1.x host does.
func (m *Manager) RestorePersisted(restore Restorer) {
	for _, saved := range m.persisted() {
		go m.restoreOne(saved, restore)
	}
}

func (m *Manager) restoreOne(saved SavedWindow, restore Restorer) {
	fresh, err := restore(saved.ExtensionID, saved.RestoreKey)
	m.onUI(func() { m.adoptRestored(saved, fresh, err) })
}

func (m *Manager) adoptRestored(saved SavedWindow, fresh *api.WindowRestore, err error) {
	if err != nil || fresh == nil {
		m.forget(saved.ExtensionID, saved.RestoreKey)
		return
	}
	options := api.WindowOptions{}
	if fresh.Options != nil {
		options = *fresh.Options
	}
	options.RestoreKey = saved.RestoreKey
	options.Persistent = true
	if err := m.Create(saved.ExtensionID, fresh.View, options); err != nil {
		m.forget(saved.ExtensionID, saved.RestoreKey)
	}
}

func (m *Manager) persisted() []SavedWindow {
	state, err := m.loadSaved()
	if err != nil {
		return nil
	}
	return state.Windows
}

// BoundRestorer returns the installation authority captured with the restoring runtime.
type BoundRestorer func(extID, restoreKey string) (*api.WindowRestore, Authority, error)

// RestorePersistedBound retains saved records when the owning installation is unavailable or replaced.
func (m *Manager) RestorePersistedBound(restore BoundRestorer) {
	for _, saved := range m.persisted() {
		go func(saved SavedWindow) {
			fresh, authority, err := restore(saved.ExtensionID, saved.RestoreKey)
			m.onUI(func() { m.adoptRestoredBound(saved, fresh, authority, err) })
		}(saved)
	}
}

func (m *Manager) adoptRestoredBound(saved SavedWindow, fresh *api.WindowRestore, authority Authority, err error) {
	if err != nil || authority == nil || !authority.Valid() {
		return
	}
	if fresh == nil {
		m.update(func(state *Saved) {
			if authority.Valid() {
				state.Windows = withoutRestoreKey(state.Windows, saved.ExtensionID, saved.RestoreKey)
			}
		})
		return
	}
	options := api.WindowOptions{}
	if fresh.Options != nil {
		options = *fresh.Options
	}
	options.RestoreKey, options.Persistent = saved.RestoreKey, true
	// A failed or revoked restore must not erase the stored window.
	_ = m.createBound(saved.ExtensionID, authority, fresh.View, options)
}
