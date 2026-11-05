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
	tray := &Tray{options: opts, title: opts.Title, toolTip: opts.ToolTip}
	h.trays = append(h.trays, tray)
	return tray, nil
}

// Trays returns every item created so far, in creation order.
func (h *TrayHost) Trays() []*Tray {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.trays)
}

// Tray is a fake menu bar item that records its title, icon, menu and closed state. Its bar is light until
// SetDark, and SetDark's appearance change runs once ChangeAppearance is called, as the OS would.
type Tray struct {
	mu         sync.Mutex
	options    platform.TrayOptions
	title      string
	toolTip    string
	icon       []byte
	template   bool
	dark       bool
	appearance []func()
	items      []platform.MenuItem
	closed     bool
	primary    func()
	secondary  func()
	popUps     [][]platform.MenuItem
	popovers   []*Popover
}

var _ platform.Tray = (*Tray)(nil)

// OnPrimaryClick records fn, which Click runs while the item has no menu.
func (t *Tray) OnPrimaryClick(fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.primary = fn
}

// OnSecondaryClick records fn, which RightClick runs.
func (t *Tray) OnSecondaryClick(fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.secondary = fn
}

// PopUpMenu records the rows shown as a context menu. Nothing is recorded for no rows.
func (t *Tray) PopUpMenu(items []platform.MenuItem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(items) > 0 && !t.closed {
		t.popUps = append(t.popUps, slices.Clone(items))
	}
}

// OpenPopover makes a hidden popover of the given size. The content is not used by the fake.
func (t *Tray) OpenPopover(opts platform.PopoverOptions, _ any) (platform.Popover, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	popover := &Popover{options: opts}
	t.popovers = append(t.popovers, popover)
	return popover, nil
}

// Popovers returns the popovers opened on the item, in order.
func (t *Tray) Popovers() []*Popover {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]*Popover(nil), t.popovers...)
}

// PopUps returns the context menus shown, in order.
func (t *Tray) PopUps() [][]platform.MenuItem {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.popUps)
}

// Click runs the primary click handler, as a click on the icon does. It does nothing while the item has an attached
// menu, which the OS shows instead.
func (t *Tray) Click() {
	t.mu.Lock()
	fn := t.primary
	attached := len(t.items) > 0
	t.mu.Unlock()
	if fn != nil && !attached {
		fn()
	}
}

// RightClick runs the secondary click handler, as a right click on the icon does.
func (t *Tray) RightClick() {
	t.mu.Lock()
	fn := t.secondary
	t.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Options returns the options the item was created with.
func (t *Tray) Options() platform.TrayOptions {
	return t.options
}

// SetToolTip records the new hover text.
func (t *Tray) SetToolTip(tip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.toolTip = tip
	}
}

// ToolTip returns the hover text last set, or the one the item was created with.
func (t *Tray) ToolTip() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.toolTip
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

// SetIcon records the PNG and whether it is a template. An empty PNG leaves the icon as it is.
func (t *Tray) SetIcon(png []byte, template bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed && len(png) > 0 {
		t.icon, t.template = slices.Clone(png), template
	}
	return nil
}

// Icon returns the PNG last set and whether it is a template icon.
func (t *Tray) Icon() ([]byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.icon), t.template
}

// Dark reports the bar's appearance set with SetDark.
func (t *Tray) Dark() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dark
}

// SetDark sets whether the bar reads as dark; the appearance callbacks run on ChangeAppearance.
func (t *Tray) SetDark(dark bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.dark = dark
}

// OnAppearanceChange records fn to run on ChangeAppearance.
func (t *Tray) OnAppearanceChange(fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.appearance = append(t.appearance, fn)
}

// ChangeAppearance runs the appearance callbacks, as the OS does when the bar turns dark or light. They run
// without the lock held, so they may call back into the item.
func (t *Tray) ChangeAppearance() {
	t.mu.Lock()
	callbacks := slices.Clone(t.appearance)
	t.mu.Unlock()
	for _, fn := range callbacks {
		fn()
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
