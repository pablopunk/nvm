package app

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/store"
)

// requestQuit asks the platform to quit the app. Quit Nevermind, the menu bar's Quit and the extension quit op all
// come here, so every quit takes the same path through the windows and the quit listeners.
func (p *App) requestQuit() {
	p.ports.System.Quit()
}

// beginQuit runs when a quit starts, before any window closes. The flag tells the windows that their close is a quit.
func (p *App) beginQuit(*mygo.QuitEvent) {
	p.quitting = true
	if p.menuCancel != nil {
		p.menuCancel()
	}
	if p.menus != nil {
		p.menus.Close()
	}
	p.stopUpdateChecks(nil)
	if p.extWindows != nil {
		p.extWindows.BeginQuit()
	}
	p.clip.setEnabled(false)
	p.clip.flush()
}

// hidePanelUnlessQuitting keeps the palette open for the next summon when its window is closed. A quit closes it for
// real: MyGo cancels the whole quit when a window refuses its close, so a refusal here would stop every quit.
func (p *App) hidePanelUnlessQuitting(e *mygo.CloseEvent) {
	if p.quitting {
		return
	}
	e.PreventDefault()
	p.Hide()
}

// closeMenubarItems removes the menu bar item and the items of the extensions. It runs before the event loop stops,
// because MyGo drops the tray calls made after that.
func (p *App) closeMenubarItems(*mygo.QuitEvent) {
	p.menubar.Close()
	if service := p.menubarItems.Load(); service != nil {
		service.CloseAll()
	}
}

// closeStoreOnQuit returns the quit listener that closes st. Close writes what is pending and releases the store's
// lease, so a second instance can take the directory at once.
func closeStoreOnQuit(st *store.Store) func() {
	return func() { _ = st.Close() }
}
