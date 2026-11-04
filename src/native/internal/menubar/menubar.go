// Package menubar is the app's own menu bar or tray item: its icon, the title a running timer shows, and the menu
// with Open, Check for Updates, Settings and Quit, in the order of the 1.x menu bar. It holds no palette state. The
// composition root supplies the actions and the UI-thread dispatcher, because a click arrives on the platform's
// goroutine.
//
// Layer: service tier (with jobs, shortcuts, windowmgmt, dictation, update and diag). It imports platform only.
package menubar

import "github.com/pablopunk/nvm/native/internal/platform"

// Tooltip is the text the item shows when the pointer rests on it.
const Tooltip = "Nevermind"

// Actions are the menu's rows. A nil action leaves its row in the menu with nothing to do.
type Actions struct {
	Open            func()
	CheckForUpdates func()
	Settings        func()
	Quit            func()
}

// Options wire an item. OnUI runs a function on the UI thread; rows call their action through it.
type Options struct {
	Host    platform.TrayHost
	Actions Actions
	OnUI    func(fn func())
}

// Item is the app's menu bar item. A nil *Item ignores every call, so features need no check when no tray exists.
type Item struct {
	tray platform.Tray
}

// New creates the item with its icon and menu. A host that cannot make an item returns its error, and the app runs
// without a menu bar item.
func New(opts Options) (*Item, error) {
	tray, err := opts.Host.NewTray(platform.TrayOptions{ID: "nevermind", ToolTip: Tooltip, Icon: Icon()})
	if err != nil {
		return nil, err
	}
	tray.SetMenu(Rows(opts.Actions, opts.OnUI))
	return &Item{tray: tray}, nil
}

// SetTitle shows text next to the icon. An empty title shows only the icon.
func (m *Item) SetTitle(title string) {
	if m == nil {
		return
	}
	m.tray.SetTitle(title)
}

// Close removes the item from the menu bar or tray.
func (m *Item) Close() {
	if m == nil {
		return
	}
	m.tray.Close()
}

// Rows is the menu, in the order the 1.x menu bar shows it: Open, a divider, updates and settings, a divider, Quit.
// Each action runs on the UI thread when onUI is set, and directly otherwise.
func Rows(a Actions, onUI func(fn func())) []platform.MenuItem {
	return []platform.MenuItem{
		row("open", "Open Nevermind", a.Open, onUI),
		{Separator: true},
		row("check-updates", "Check for Updates…", a.CheckForUpdates, onUI),
		row("settings", "Settings…", a.Settings, onUI),
		{Separator: true},
		row("quit", "Quit Nevermind", a.Quit, onUI),
	}
}

// row is one menu entry whose click runs fn on the UI thread.
func row(id, label string, fn func(), onUI func(fn func())) platform.MenuItem {
	return platform.MenuItem{ID: id, Label: label, Action: uiAction(fn, onUI)}
}

// uiAction wraps fn so it runs through onUI. A nil fn stays nil, which the port treats as no action.
func uiAction(fn func(), onUI func(fn func())) func() {
	if fn == nil {
		return nil
	}
	if onUI == nil {
		return fn
	}
	return func() { onUI(fn) }
}
