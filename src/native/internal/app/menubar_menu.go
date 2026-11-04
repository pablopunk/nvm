package app

import (
	"runtime"
	"strings"

	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/update"
)

// menubarMenu is the menu bar's rows as the app holds them now: the palette shortcut, and the update row for the
// updater's state.
func (p *App) menubarMenu() menubar.Menu {
	return menubar.Menu{
		GOOS:        p.menuPlatform(),
		Accelerator: p.shortcutsManager().PaletteAccelerator(),
		Update:      p.menubarUpdateRow(p.updateSnapshot().State),
	}
}

// menuPlatform is the platform the menu bar draws its rows for. Tests set menuOS to draw them as macOS, where the rows
// carry the shortcut and the download glyph.
func (p *App) menuPlatform() string {
	if p.menuOS != "" {
		return p.menuOS
	}
	return runtime.GOOS
}

// refreshMenubarMenu redraws the menu bar's rows when the palette shortcut or the update changes.
func (p *App) refreshMenubarMenu() {
	if p.menubar != nil {
		p.menubar.SetMenu(menubar.Rows(p.menubarActions(), p.onUI, p.menubarMenu()))
	}
}

// menubarUpdateRow is the menu's update row for an update's state, or nil when Check for Updates… stays in its place.
// Each state reads as 1.x reads it: an update found or downloading is offered as a download, a ready one as an install,
// and a failed install as a download to try again. A failed check leaves Check for Updates….
func (p *App) menubarUpdateRow(state update.State) *menubar.UpdateRow {
	switch state.Status {
	case update.StatusDownloaded:
		return &menubar.UpdateRow{Label: updateTitle("Install", state), Action: p.installFromMenu}
	case update.StatusAvailable, update.StatusDownloading:
		return &menubar.UpdateRow{Label: updateTitle("Download", state), Action: p.openUpdatesAndCheck}
	case update.StatusInstalling:
		return &menubar.UpdateRow{Label: updateTitle("Installing", state), Disabled: true}
	case update.StatusFailed:
		if state.Stage == update.StageInstall && state.LatestVersion != "" {
			return &menubar.UpdateRow{Label: updateTitle("Download", state), Action: p.openUpdatesAndCheck}
		}
	}
	return nil
}

// updateTitle is a menu title such as "Install Nevermind 2.1.0", without a version when the updater has none.
func updateTitle(verb string, state update.State) string {
	return strings.TrimSpace(verb + " Nevermind " + state.LatestVersion)
}

// openUpdatesAndCheck opens the Updates page and checks, as Check for Updates… and the download row do in 1.x. A check
// or download already running refuses the new one, so the page shows the one in progress.
func (p *App) openUpdatesAndCheck() {
	p.openUpdatesPage()
	p.checkForUpdates(func(string) {})
}

// installFromMenu opens the Updates page and restarts into the downloaded update, as the install row does in 1.x.
func (p *App) installFromMenu() {
	p.openUpdatesPage()
	_ = p.restartToUpdate()
}

// openUpdatesPage shows the Updates page in the palette and gives it the keyboard.
func (p *App) openUpdatesPage() {
	p.Open(p.updatesView())
	p.focusPanel()
}
