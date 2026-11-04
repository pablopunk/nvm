package app

import (
	"context"
	"errors"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/menubar/extitems"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// setUpMenubar creates the app's menu bar item over the platform's tray host, and the scheduler that renders every
// status item. A platform without a tray host runs the app without an item; the palette still opens by its shortcut.
func (p *App) setUpMenubar() {
	p.menubarJobs = menubar.NewScheduler(real.NewPorts().Clock, p.onUI)
	item, err := menubar.New(menubar.Options{Host: p.ports.Tray, OnUI: p.onUI, Actions: p.menubarActions()})
	if err != nil {
		logx.Default().Warn("menu bar item unavailable", logx.F{"error": err.Error()})
	} else {
		p.menubar = item
	}
	p.startExtensionItems()
}

// startExtensionItems starts the service that shows the menu bar items of extensions. It runs with the scheduler, so
// its items appear once the extension host has started and close when an extension is disabled.
func (p *App) startExtensionItems() {
	service := extitems.New(extitems.Options{
		Host:      p.ports.Tray,
		Scheduler: p.menubarJobs,
		OnUI:      p.onUI,
		Source:    extensionMenuBars{p: p},
		Hooks: extitems.Hooks{
			OpenURL:        func(url string) { _ = mygo.Shell.OpenExternal(url) },
			CopyText:       func(text string) { mygo.Clipboard.WriteText(text) },
			ShowExtensions: func(string) { p.Open(p.extensionsView()); p.focusPanel() },
		},
	})
	p.menubarItems.Store(service)
	service.Start()
}

// refreshMenuBar re-renders the menu bar items of one extension. An extension's RefreshMenuBar call arrives on the
// child's exchange goroutine, so it only schedules the render.
func (p *App) refreshMenuBar(extensionID string) {
	if service := p.menubarItems.Load(); service != nil {
		service.Refresh(extensionID)
	}
}

// runMenubarScheduler drives the scheduler until the app quits.
func (p *App) runMenubarScheduler() {
	go p.menubarJobs.Run(context.Background())
}

// errLoading answers a read that came before the extension host started; the next sync finds the extensions then.
var errLoading = errors.New("extensions are still loading")

// extensionMenuBars reads the menu bar items of the running extensions. Until the extension host has started there
// are none, so the sync finds nothing and the items appear on a later sync.
type extensionMenuBars struct{ p *App }

func (e extensionMenuBars) MenuBarExtensions() []extitems.Extension {
	m := e.p.extensionManager()
	if m == nil {
		return nil
	}
	var listed []extitems.Extension
	for _, ext := range m.MenuBarExtensions() {
		listed = append(listed, extitems.Extension{ID: ext.ID, Revision: ext.Revision})
	}
	return listed
}

func (e extensionMenuBars) MenuBars(ctx context.Context, id string) ([]api.MenuBar, *api.Failure, error) {
	if m := e.p.extensionManager(); m != nil {
		return m.MenuBars(ctx, id)
	}
	return nil, nil, errLoading
}

func (e extensionMenuBars) MenuBarTriggers(ctx context.Context, id string) ([]api.Trigger, *api.Failure, error) {
	if m := e.p.extensionManager(); m != nil {
		return m.MenuBarTriggers(ctx, id)
	}
	return nil, nil, errLoading
}

func (e extensionMenuBars) Run(ctx context.Context, id string, in api.Invocation) (api.Result, *api.Failure, error) {
	if m := e.p.extensionManager(); m != nil {
		return m.Run(ctx, id, in)
	}
	return api.Result{}, nil, errLoading
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

// setTrayTitle shows text next to the app's menu bar icon, as the running timer does. An empty title shows only the icon.
func (p *App) setTrayTitle(s string) {
	p.menubar.SetTitle(s)
}
