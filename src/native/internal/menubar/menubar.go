// Package menubar is the menu bar or tray item the host keeps: an item with a title, an icon and a menu, made over
// platform.TrayHost. The app's own item (Open, Check for Updates, Settings, Quit, and the timer title) and the items
// extensions declare (package menubar/extitems) are both made here, so every status item shares one path to the OS.
// The Scheduler in schedule.go runs the renders off the UI thread on their own cadence.
//
// It holds no palette state. The composition root supplies the actions and the UI-thread dispatcher, because a click
// arrives on the platform's goroutine.
//
// Layer: service tier (with jobs, shortcuts, windowmgmt, dictation, update and diag). It imports platform only.
package menubar

import (
	"runtime"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Tooltip is the text the app's item shows when the pointer rests on it.
const Tooltip = "Nevermind"

// Actions are the app's menu rows. A nil action leaves its row in the menu with nothing to do.
type Actions struct {
	Open            func()
	CheckForUpdates func()
	Settings        func()
	Quit            func()
}

// Options wire the app's item. OnUI runs a function on the UI thread; rows call their action through it. IconStyle
// is the menuBarIconStyle setting; empty or unknown draws DefaultIconStyle.
type Options struct {
	Host      platform.TrayHost
	Actions   Actions
	OnUI      func(fn func())
	IconStyle string
	Menu      Menu
}

// Item is one menu bar or tray item. A nil *Item ignores every call, so features need no check when no tray exists.
// Its glyph follows the bar: it is drawn again when the bar turns dark or light.
type Item struct {
	tray platform.Tray
	goos string
	// glyph names the extension glyph an item draws instead of an app style; empty for the app's own item.
	glyph string

	mu      sync.Mutex
	style   string
	title   string
	rows    []platform.MenuItem
	primary func()
}

// NewItem creates an item over host, with its glyph drawn in style (see ChooseIcon). A host that cannot make one
// returns its error, and the caller runs without it.
func NewItem(host platform.TrayHost, options platform.TrayOptions, style string) (*Item, error) {
	return newItem(host, options, &Item{style: NormalizeIconStyle(style), goos: runtime.GOOS})
}

// NewGlyphItem creates an item that draws the extension glyph named glyph on goos, with its title and tooltip as
// StatusText lays them out for goos.
func NewGlyphItem(host platform.TrayHost, options platform.TrayOptions, glyph, goos string) (*Item, error) {
	options.Title, options.ToolTip = StatusText(goos, options.Title, options.ToolTip)
	return newItem(host, options, &Item{style: DefaultIconStyle, goos: goos, glyph: glyph})
}

func newItem(host platform.TrayHost, options platform.TrayOptions, item *Item) (*Item, error) {
	item.title = options.Title
	options.Title = TitleFor(item.style, item.goos, options.Title)
	tray, err := host.NewTray(options)
	if err != nil {
		return nil, err
	}
	item.tray = tray
	tray.OnPrimaryClick(item.primaryClicked)
	tray.OnSecondaryClick(item.secondaryClicked)
	if err := item.applyIcon(); err != nil {
		tray.Close()
		return nil, err
	}
	tray.OnAppearanceChange(item.refreshIcon)
	return item, nil
}

// New creates the app's item with its icon and menu. A host that cannot make an item returns its error, and the app
// runs without a menu bar item.
func New(opts Options) (*Item, error) {
	item, err := NewItem(opts.Host, platform.TrayOptions{ID: "nevermind", ToolTip: Tooltip}, opts.IconStyle)
	if err != nil {
		return nil, err
	}
	item.SetMenu(Rows(opts.Actions, opts.OnUI, opts.Menu))
	return item, nil
}

// SetToolTip changes the hover text of the item in place.
func (m *Item) SetToolTip(tip string) {
	if m == nil {
		return
	}
	m.tray.SetToolTip(tip)
}

// SetTitle shows text next to the icon. An empty title shows only the icon.
func (m *Item) SetTitle(title string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.title = title
	m.mu.Unlock()
	m.renderTitle()
}

// SetIconStyle changes the glyph to style, as the menuBarIconStyle setting does. An empty or unknown style draws
// DefaultIconStyle.
func (m *Item) SetIconStyle(style string) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.style = NormalizeIconStyle(style)
	m.mu.Unlock()
	m.renderTitle()
	return m.applyIcon()
}

// SetMenu replaces the menu the item shows on click. An empty list removes it.
func (m *Item) SetMenu(rows []platform.MenuItem) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.rows = rows
	m.mu.Unlock()
	m.applyMenu()
}

// Tray is the platform item the menu bar item wraps, for the popovers an extension opens under it.
func (m *Item) Tray() platform.Tray {
	if m == nil {
		return nil
	}
	return m.tray
}

// SetPrimaryAction makes a click on the icon run fn, as an extension popover opens, and moves the menu to a right
// click. Nil puts the menu back on the click.
func (m *Item) SetPrimaryAction(fn func()) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.primary = fn
	m.mu.Unlock()
	m.applyMenu()
}

// applyMenu attaches the menu to the icon, unless a primary action owns the click.
func (m *Item) applyMenu() {
	m.mu.Lock()
	rows, primary := m.rows, m.primary
	m.mu.Unlock()
	if primary != nil {
		m.tray.SetMenu(nil)
		return
	}
	m.tray.SetMenu(rows)
}

// primaryClicked runs the primary action, if one is set. The platform calls it on its own thread.
func (m *Item) primaryClicked() {
	m.mu.Lock()
	fn := m.primary
	m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// secondaryClicked shows the menu at the pointer while a primary action owns the click.
func (m *Item) secondaryClicked() {
	m.mu.Lock()
	rows, primary := m.rows, m.primary
	m.mu.Unlock()
	if primary != nil {
		m.tray.PopUpMenu(rows)
	}
}

// Close removes the item from the menu bar or tray.
func (m *Item) Close() {
	if m == nil {
		return
	}
	m.tray.Close()
}

// snapshot reads the style and title under the lock. The tray is called without it: the bar's callbacks arrive on
// the main thread and read the style too.
func (m *Item) snapshot() (style, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.style, m.title
}

// applyIcon draws the item's glyph for the bar as it is now.
func (m *Item) applyIcon() error {
	style, _ := m.snapshot()
	dark := m.tray.Dark()
	choice := ChooseIcon(style, m.goos, dark)
	if m.glyph != "" {
		choice = ChooseGlyph(m.glyph, m.goos, dark)
	}
	return m.tray.SetIcon(choice.PNG, choice.Template)
}

// renderTitle shows the title the current style gives it.
func (m *Item) renderTitle() {
	style, title := m.snapshot()
	m.tray.SetTitle(TitleFor(style, m.goos, title))
}

// refreshIcon redraws the glyph when the bar's appearance changes. The OS calls it without a caller to report to.
func (m *Item) refreshIcon() {
	_ = m.applyIcon()
}
