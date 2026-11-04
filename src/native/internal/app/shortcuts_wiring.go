package app

import (
	"log"
	"runtime"
	"time"

	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/store"
)

const shortcutsFile = "shortcuts.json"

// The palette key may still be held by a previous instance, so it is retried for a few seconds before the app gives
// up on it.
const (
	paletteBindAttempts = 8
	paletteBindDelay    = 400 * time.Millisecond
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
		GOOS:      runtime.GOOS,
		Commands:  p.shortcutCommands(),
		Run:       p.runShortcutCommand,
		OnPalette: func() { p.Toggle() },
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
	return actions
}

func shortcutCommandID(title string) string {
	return "command:" + title
}

// runShortcutCommand opens the palette and runs the command a shortcut names, as choosing it from the root does.
func (p *App) runShortcutCommand(actionID string) {
	for _, cmd := range p.commands(false) {
		if !cmd.Hidden && shortcutCommandID(cmd.Title) == actionID {
			p.Show()
			cmd.Run(search.Host(p))
			return
		}
	}
}

// startShortcuts binds the palette hotkey and the actions. It runs once the palette exists. A recovery key is only
// accepted after the chosen key has been refused on every attempt, because a previous instance may release it soon.
func (p *App) startShortcuts() {
	status := p.shortcutsManager().Start()
	for attempt := 1; !status.Registered && attempt < paletteBindAttempts; attempt++ {
		time.Sleep(paletteBindDelay)
		status = p.shortcutsManager().Start()
	}
	switch {
	case status.Registered:
	case status.Recovery != "":
		log.Println("shortcut", status.Accelerator, "is in use; bound", status.Recovery, "instead")
	default:
		log.Println("shortcut", status.Accelerator, "is in use by another app")
	}
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
