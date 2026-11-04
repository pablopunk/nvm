package fake

import (
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpHotkeysRegister is the operation name accepted by Hotkeys.FailNext.
const OpHotkeysRegister = "Register"

// Hotkeys records registrations and lets a test fire them, as the user pressing the shortcut would. Like the
// OS, it refuses a second registration of an accelerator already held, with ErrConflict.
//
//	keys := fake.NewHotkeys()
//	keys.Register("Command+Alt+K", summon)
//	keys.Fire("Command+Alt+K") // runs summon
type Hotkeys struct {
	mu       sync.Mutex
	handlers map[string]func()
	faults   faultSet
}

var _ platform.Hotkeys = (*Hotkeys)(nil)

// NewHotkeys returns an empty registry.
func NewHotkeys() *Hotkeys {
	return &Hotkeys{handlers: map[string]func(){}}
}

// FailNext makes the next Register return err.
func (h *Hotkeys) FailNext(op string, err error) {
	h.faults.failNext(op, err)
}

// Register binds accelerator to fn.
func (h *Hotkeys) Register(accelerator string, fn func()) error {
	if err := h.faults.take(OpHotkeysRegister); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, taken := h.handlers[accelerator]; taken {
		return fmt.Errorf("%w: %s", platform.ErrConflict, accelerator)
	}
	h.handlers[accelerator] = fn
	return nil
}

// Unregister removes the binding, if any.
func (h *Hotkeys) Unregister(accelerator string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.handlers, accelerator)
}

// Registered lists the bound accelerators, sorted.
func (h *Hotkeys) Registered() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	names := make([]string, 0, len(h.handlers))
	for name := range h.handlers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Fire runs the handler bound to accelerator on the caller's goroutine. It returns ErrNotFound when nothing
// is bound. The handler runs without the lock held, so it may register or unregister shortcuts.
func (h *Hotkeys) Fire(accelerator string) error {
	h.mu.Lock()
	fn, ok := h.handlers[accelerator]
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %s", platform.ErrNotFound, accelerator)
	}
	fn()
	return nil
}
