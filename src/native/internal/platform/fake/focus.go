package fake

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Focus holds one active window and records the windows the palette gave the keyboard back to. A test sets the
// active window before it summons the palette, then checks Restored after the palette hides.
type Focus struct {
	mu        sync.Mutex
	active    platform.FocusTarget
	hasActive bool
	restored  []platform.FocusTarget
}

var _ platform.Focus = (*Focus)(nil)

// NewFocus returns a focus with no active window.
func NewFocus() *Focus { return &Focus{} }

// SetActive makes target the window that has the keyboard.
func (f *Focus) SetActive(target platform.FocusTarget) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active, f.hasActive = target, true
}

// ClearActive leaves no window with the keyboard.
func (f *Focus) ClearActive() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hasActive = false
}

// Active returns the window with the keyboard, or ErrNotFound when none has it.
func (f *Focus) Active() (platform.FocusTarget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hasActive {
		return platform.FocusTarget{}, platform.ErrNotFound
	}
	return f.active, nil
}

// Restore records target as the window the keyboard went back to, and makes it the active one.
func (f *Focus) Restore(target platform.FocusTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restored = append(f.restored, target)
	f.active, f.hasActive = target, true
	return nil
}

// Restored lists the windows the keyboard was given back to, in order.
func (f *Focus) Restored() []platform.FocusTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]platform.FocusTarget(nil), f.restored...)
}
