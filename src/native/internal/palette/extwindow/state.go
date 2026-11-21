package extwindow

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Saved is what the host keeps about extension windows between launches: the persistent windows to restore and the
// frames that windows remember, keyed by extension and restore key.
type Saved struct {
	Windows []SavedWindow    `json:"windows,omitempty"`
	Frames  map[string]Frame `json:"frames,omitempty"`
}

// SavedWindow is one persistent window: the extension that owns it, the key it is restored by, and the options it
// opened with. The view is not saved; the extension builds it again on relaunch.
type SavedWindow struct {
	ExtensionID string            `json:"extensionId"`
	RestoreKey  string            `json:"restoreKey"`
	Options     api.WindowOptions `json:"options"`
}

// Saver reads and writes Saved. A store collection of Saved satisfies it.
type Saver interface {
	Load() (Saved, error)
	Update(change func(*Saved)) error
}
