package app

import (
	"runtime"

	"github.com/pablopunk/nvm/native/internal/palette/windows"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/store"
)

const shortcutsFile = "shortcuts.json"

// shortcutsManager returns the app's shortcut manager, creating it on first use.
func (p *App) shortcutsManager() *shortcuts.Manager {
	if p.shortcutMgr == nil {
		p.shortcutMgr = shortcuts.NewManager(p.shortcutsSetup())
	}
	return p.shortcutMgr
}

func (p *App) shortcutsSetup() shortcuts.Setup {
	return shortcuts.Setup{
		Doc:       p.shortcutsDocument(),
		Hotkeys:   p.ports.Hotkeys,
		Clock:     p.ports.Clock,
		GOOS:      runtime.GOOS,
		Commands:  p.shortcutCommands(),
		Run:       p.runShortcutCommand,
		OnPalette: func() { p.Toggle() },
		OnSummon:  p.notifySummon,
	}
}

// shortcutsDocument stores the choices in shortcuts.json. The palette key of a fresh install is the one the settings
// hold, so a Hotkey already chosen in settings.json carries over.
func (p *App) shortcutsDocument() shortcuts.Persistence {
	legacyPalette := p.S.Hotkey
	defaults := func() shortcuts.Document { return shortcuts.DefaultDocument(legacyPalette) }
	if p.userStore == nil || p.userReadOnly {
		return shortcuts.NewMemoryDocument(defaults())
	}
	return store.Doc(p.userStore, shortcutsFile, 1, defaults)
}

// shortcutCommands lists the commands that take a shortcut: the root commands that are visible.
func (p *App) shortcutCommands() []shortcuts.Action {
	var actions []shortcuts.Action
	for _, cmd := range p.commands(false) {
		if !cmd.Hidden {
			actions = append(actions, shortcuts.Action{ID: shortcutCommandID(cmd.Title), Title: cmd.Title})
		}
	}
	if p.windowManagementEnabled() {
		actions = append(actions, windows.ShortcutActions()...)
	}
	return actions
}

func shortcutCommandID(title string) string {
	return "command:" + title
}

// runShortcutCommand opens the palette and runs the command a shortcut names, as choosing it from the root does.
func (p *App) runShortcutCommand(actionID string) {
	if windows.RunShortcut(p.windowDeps(), actionID) {
		return
	}
	for _, cmd := range p.commands(false) {
		if !cmd.Hidden && shortcutCommandID(cmd.Title) == actionID {
			p.Show()
			p.trackedRun(cmd)(search.Host(p))
			return
		}
	}
}

// startShortcuts binds the palette shortcut and the actions. It runs once the palette exists. Summon retries a shortcut
// that a previous instance may still hold, and the banner says when it stays unavailable.
func (p *App) startShortcuts() {
	p.summonChanged(p.shortcutsManager().Start())
}

// onPaletteHide closes the app's panels and gives back the shortcuts a recorder holds, so the palette key still works.
func (p *App) onPaletteHide() {
	p.hideApps()
	p.resumeShortcutsIfSuspended()
}

// resumeShortcutsIfSuspended binds the shortcuts again when the palette hides while a recorder is open, so the palette
// key still opens it.
func (p *App) resumeShortcutsIfSuspended() {
	if p.shortcutMgr != nil && p.shortcutMgr.Suspended() {
		p.shortcutMgr.Resume()
	}
}
