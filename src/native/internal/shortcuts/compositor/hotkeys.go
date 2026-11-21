package compositor

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ownedHotkeys registers through the portal only while the compositor does not own the shortcuts. It reports every
// change to the set of bound accelerators, so the module can follow the user's shortcuts.
type ownedHotkeys struct {
	inner    platform.Hotkeys
	owns     func() bool
	onChange func()
	mu       sync.Mutex
	bound    map[string]bool
	portal   map[string]bool
}

// Hotkeys wraps inner for the shortcut manager. onChange runs after the bound set changes; it must return quickly.
func (s *Service) Hotkeys(inner platform.Hotkeys, onChange func()) platform.Hotkeys {
	return &ownedHotkeys{
		inner: inner, owns: func() bool { return s.State().OwnsShortcuts }, onChange: onChange,
		bound: map[string]bool{}, portal: map[string]bool{},
	}
}

// Register binds the accelerator through the portal unless the compositor runs it. A compositor-owned accelerator
// reports success: the compositor's module binds it, and Apply reported whether that module is loaded.
func (h *ownedHotkeys) Register(accelerator string, fn func()) error {
	owned := h.owns()
	if !owned {
		if err := h.inner.Register(accelerator, fn); err != nil {
			return err
		}
	}
	h.mu.Lock()
	changed := !h.bound[accelerator]
	h.bound[accelerator] = true
	h.portal[accelerator] = !owned
	h.mu.Unlock()
	if changed && h.onChange != nil {
		h.onChange()
	}
	return nil
}

// Unregister releases the accelerator from the portal if it was bound there.
func (h *ownedHotkeys) Unregister(accelerator string) {
	h.mu.Lock()
	changed := h.bound[accelerator]
	viaPortal := h.portal[accelerator]
	delete(h.bound, accelerator)
	delete(h.portal, accelerator)
	h.mu.Unlock()
	if viaPortal {
		h.inner.Unregister(accelerator)
	}
	if changed && h.onChange != nil {
		h.onChange()
	}
}
