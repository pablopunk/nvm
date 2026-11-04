package app

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/menubar"
)

// setUpMenubar creates the app's menu bar item over the platform's tray host. A platform without one runs the app
// without an item; the palette still opens by its shortcut.
func (p *App) setUpMenubar() {
	item, err := menubar.New(menubar.Options{Host: p.ports.Tray, OnUI: p.onUI, Actions: p.menubarActions()})
	if err != nil {
		logx.Default().Warn("menu bar item unavailable", logx.F{"error": err.Error()})
		return
	}
	p.menubar = item
}

// menubarActions maps the menu's rows onto the app. Each runs on the UI thread through the menu's dispatcher.
func (p *App) menubarActions() menubar.Actions {
	return menubar.Actions{
		Open:            func() { p.Show(); p.focusPanel() },
		CheckForUpdates: func() { p.checkForUpdates(p.notifyUpdate) },
		Settings:        func() { p.Open(p.settingsView()); p.focusPanel() },
		Quit:            func() { mygo.App.Quit() },
	}
}

// focusPanel gives the palette keyboard focus. A menu closes after its click and hands focus back to the window that
// had it, so a palette shown from the menu without asking for focus hides on blur at once.
func (p *App) focusPanel() {
	if p.panel != nil {
		p.panel.Focus()
	}
}

// setTrayTitle shows text next to the menu bar icon, as the running timer does. An empty title shows only the icon.
func (p *App) setTrayTitle(s string) {
	p.menubar.SetTitle(s)
}
