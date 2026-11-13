package app

import (
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// actionRunnable reports whether the action command line can run actionID now: the same lookup runActionCommandOnUI
// does, without running anything. A shortcut is bound to the compositor only for an action that passes, so a bound key
// never answers "no action has the ID". It runs on the UI thread.
func (p *App) actionRunnable(actionID string) bool {
	if actionID == shortcuts.PaletteID || actionID == shortcuts.PaletteToggleID {
		return true
	}
	if path, ok := shortcuts.AppPathOf(actionID); ok {
		return p.appInstalled(path)
	}
	if entry, ok := p.extensionShortcutCommand(actionID); ok {
		return p.extensionMayRun(entry.extID)
	}
	if extID, _, ok := splitExtensionCommandShortcutID(actionID); ok {
		return p.extensionAwaitingCommands(extID)
	}
	_, ok := p.shortcutAction(actionID)
	return ok
}

// extensionAwaitingCommands reports whether extID may run but has not listed its commands yet. The command line asks
// such an extension for its commands before it rejects, so its shortcuts stay bound until the listing is known.
func (p *App) extensionAwaitingCommands(extID string) bool {
	m := p.extensionManager()
	if m == nil || !p.extensionMayRun(extID) {
		return false
	}
	known := false
	for _, result := range m.CachedCommands() {
		known = known || result.ExtensionID == extID
	}
	if known {
		return false
	}
	for _, listing := range m.List() {
		if listing.ID == extID {
			return true
		}
	}
	return m.IsBuiltIn(extID)
}

// runnableActions keeps the bound actions the command line can run.
func (p *App) runnableActions(actions map[string]string) map[string]string {
	out := make(map[string]string, len(actions))
	for actionID, accelerator := range actions {
		if p.actionRunnable(actionID) {
			out[actionID] = accelerator
		}
	}
	return out
}

// appInstalled reports whether the app a shortcut opens exists on this machine. Shortcuts sync between machines, so a
// macOS app path can reach Linux, where binding it would give a key that only answers "not installed". The file is
// checked rather than the app index, which is still empty right after a cold start.
func (p *App) appInstalled(path string) bool {
	if p.ports.FS == nil {
		return false
	}
	_, err := p.ports.FS.Stat(path)
	return err == nil
}
