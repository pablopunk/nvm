package fake

import (
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// OpTrayNewTray is the operation name accepted by TrayHost.FailNext.
const OpTrayNewTray = "NewTray"

// TrayHost creates fake tray items and keeps every one it made, closed or not, so tests can check that a
// disabled extension's item was removed.
//
//	host := fake.NewTrayHost()
//	item, _ := host.NewTray(platform.TrayOptions{ID: "clock", Title: "12:00"})
//	host.Trays()[0].Invoke("copy") // runs the menu row's action
type TrayHost struct {
	mu     sync.Mutex
	trays  []*Tray
	faults faultSet
}

var _ platform.TrayHost = (*TrayHost)(nil)

// NewTrayHost returns a host with no items.
func NewTrayHost() *TrayHost {
	return &TrayHost{}
}

// FailNext makes the next NewTray return err.
func (h *TrayHost) FailNext(op string, err error) {
	h.faults.failNext(op, err)
}

// NewTray creates and records a tray item.
func (h *TrayHost) NewTray(opts platform.TrayOptions) (platform.Tray, error) {
	if err := h.faults.take(OpTrayNewTray); err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	tray := &Tray{options: opts, title: opts.Title}
	h.trays = append(h.trays, tray)
	return tray, nil
}

// Trays returns every item created so far, in creation order.
func (h *TrayHost) Trays() []*Tray {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.trays)
}

// Tray is a fake menu bar item that records its title, menu and closed state.
type Tray struct {
	mu      sync.Mutex
	options platform.TrayOptions
	title   string
	items   []platform.MenuItem
	closed  bool
}

var _ platform.Tray = (*Tray)(nil)

// Options returns the options the item was created with.
func (t *Tray) Options() platform.TrayOptions {
	return t.options
}

// SetTitle records the new title.
func (t *Tray) SetTitle(title string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.title = title
	}
}

// SetMenu records the new menu, copying the rows.
func (t *Tray) SetMenu(items []platform.MenuItem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.items = slices.Clone(items)
	}
}

// Close marks the item removed. Later calls do nothing.
func (t *Tray) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
}

// Title returns the current title.
func (t *Tray) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

// Items returns the current menu rows.
func (t *Tray) Items() []platform.MenuItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.items)
}

// Closed reports whether Close was called.
func (t *Tray) Closed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// Invoke runs the action of the menu row with id, as a click would. It returns ErrNotFound for an unknown or
// separator row and an error when the item is closed. The action runs without the lock held.
func (t *Tray) Invoke(id string) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return fmt.Errorf("fake tray: item is closed")
	}
	var action func()
	for _, item := range t.items {
		if item.ID == id && !item.Separator {
			action = item.Action
			break
		}
	}
	t.mu.Unlock()
	if action == nil {
		return fmt.Errorf("%w: menu row %q", platform.ErrNotFound, id)
	}
	action()
	return nil
}
