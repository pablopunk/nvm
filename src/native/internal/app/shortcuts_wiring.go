package app

import (
	"runtime"

	"github.com/pablopunk/nvm/native/internal/palette/windows"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

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
		Resolve:   p.resolveShortcutTitle,
		Declared:  p.declaredGlobalShortcuts,
		Run:       p.runShortcutCommand,
		OnPalette: func() { p.Toggle() },
		OnSummon:  p.notifySummon,
	}
}

// shortcutsDocument keeps the shortcuts in config.json, under the keys 1.x uses. Without a config store it keeps them
// for the process.
func (p *App) shortcutsDocument() shortcuts.Persistence {
	if p.sharedConfig == nil {
		return shortcuts.NewMemoryDocument(shortcuts.DefaultDocument(""))
	}
	return shortcuts.NewPortableDocument(p.sharedConfig)
}

// shortcutCommands lists the commands that take a shortcut: the root commands that are visible.
func (p *App) shortcutCommands() []shortcuts.Action {
	var actions []shortcuts.Action
	for _, cmd := range p.commands() {
		if !cmd.Hidden && cmd.ID != "" {
			actions = append(actions, shortcuts.Action{ID: cmd.ID, Title: cmd.Title})
		}
	}
	if p.windowManagementEnabled() {
		actions = append(actions, windows.ShortcutActions()...)
	}
	return actions
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
