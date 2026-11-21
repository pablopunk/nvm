package extwindow

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// onUI runs fn on the UI thread through the host's hop.
func (m *Manager) onUI(fn func()) {
	m.deps.OnUI(fn)
}

// persist keeps a persistent window's record, so the next launch restores it.
func (m *Manager) persist(w *window) {
	if !w.options.Persistent || w.options.RestoreKey == "" {
		return
	}
	entry := SavedWindow{ExtensionID: w.extID, RestoreKey: w.options.RestoreKey, Options: w.options}
	m.update(func(state *Saved) {
		state.Windows = append(withoutRestoreKey(state.Windows, w.extID, w.options.RestoreKey), entry)
	})
}

// forget drops the record of a restore key of extension extID.
func (m *Manager) forget(extID, restoreKey string) {
	m.update(func(state *Saved) {
		state.Windows = withoutRestoreKey(state.Windows, extID, restoreKey)
	})
}

// rememberFrame stores where a window that remembers its frame is now.
func (m *Manager) rememberFrame(w *window) {
	frame := w.surface.Frame()
	m.update(func(state *Saved) {
		if state.Frames == nil {
			state.Frames = map[string]Frame{}
		}
		state.Frames[frameKey(w.extID, w.options, w.id)] = frame
	})
}

// savedFrame is the frame a window that remembers its frame last had, or nil.
func (m *Manager) savedFrame(w *window) *Frame {
	state, err := m.loadSaved()
	if err != nil {
		return nil
	}
	frame, ok := state.Frames[frameKey(w.extID, w.options, w.id)]
	if !ok {
		return nil
	}
	return &frame
}

func (m *Manager) loadSaved() (Saved, error) {
	if m.deps.Saved == nil {
		return Saved{}, nil
	}
	return m.deps.Saved.Load()
}

// update applies change to the saved state. Without a saver the state lives for the session only; a write that fails
// is logged, and the window keeps working.
func (m *Manager) update(change func(*Saved)) {
	if m.deps.Saved == nil {
		return
	}
	if err := m.deps.Saved.Update(change); err != nil {
		logx.Default().Warn("extension window state not saved", logx.F{"error": err.Error()})
	}
}

// frameKey is the key a window's frame is remembered under: the restore key when it has one, else the window's id,
// both within its extension.
func frameKey(extID string, options api.WindowOptions, id string) string {
	if options.RestoreKey != "" {
		return recordKey(extID, options.RestoreKey)
	}
	return recordKey(extID, id)
}

func withoutRestoreKey(windows []SavedWindow, extID, restoreKey string) []SavedWindow {
	kept := windows[:0:0]
	for _, saved := range windows {
		if saved.ExtensionID == extID && saved.RestoreKey == restoreKey {
			continue
		}
		kept = append(kept, saved)
	}
	return kept
}
