package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// extensionCommandEntry is an extension command a shortcut can run: the extension that answers it and the command.
type extensionCommandEntry struct {
	extID   string
	command api.Command
}

// globalShortcutTable is what the shortcut manager binds from the extensions: the global shortcuts the commands declare,
// and every listed command by its shortcut key, so a key the user chose can run it too.
type globalShortcutTable struct {
	declared []shortcuts.Declared
	commands map[string]extensionCommandEntry
}

// globalShortcutTableOf builds the table from the commands the running extensions list. A failed extension lists none.
func globalShortcutTableOf(results []manager.CommandResult) globalShortcutTable {
	table := globalShortcutTable{commands: map[string]extensionCommandEntry{}}
	for _, result := range results {
		if result.Failure != nil {
			continue
		}
		for _, command := range result.Commands {
			key := commandShortcutKey(result.ExtensionID, command)
			table.commands[key] = extensionCommandEntry{extID: result.ExtensionID, command: command}
			if command.GlobalShortcut != "" {
				table.declared = append(table.declared, shortcuts.Declared{ActionID: key, Accelerator: command.GlobalShortcut, Title: command.Title})
			}
		}
	}
	return table
}

// runsWithoutPalette reports whether a command runs in place: noView and background commands never open the palette.
func runsWithoutPalette(command api.Command) bool {
	return command.Mode == api.ModeNoView || command.Mode == api.ModeBackground
}

func (p *App) declaredGlobalShortcuts() []shortcuts.Declared {
	return p.globalShortcuts.declared
}

// refreshGlobalShortcuts binds the shortcuts the running extensions declare. It reads the known commands, so it never
// waits on a child; the table is kept and bound on the UI thread.
func (p *App) refreshGlobalShortcuts() {
	m := p.extensionManager()
	if m == nil {
		return
	}
	safego.Go("shortcuts.extensions", func(context.Context) {
		table := globalShortcutTableOf(m.CachedCommands())
		p.onUI(func() {
			p.globalShortcuts = table
			p.shortcutsManager().Refresh()
		})
	})
}

// runExtensionCommandInPlace runs a command that never opens the palette. It starts off the UI thread, and the run's
// outcome is reported as a notification.
func (p *App) runExtensionCommandInPlace(entry extensionCommandEntry) {
	m := p.extensionManager()
	if m == nil {
		return
	}
	safego.Go("shortcuts.command", func(ctx context.Context) {
		result, failure, err := m.Run(ctx, entry.extID, api.Invocation{Ref: api.ActionRef{ID: entry.command.ID}, Launch: api.LaunchContext{Reason: "manual"}})
		p.reportShortcutOutcome(entry.command.Title, result, failure, err)
	})
}

// reportShortcutOutcome tells the user what a shortcut run could not do, as a notification, since the palette stays
// closed.
func (p *App) reportShortcutOutcome(title string, result api.Result, failure *api.Failure, err error) {
	if body, ok := shortcutOutcomeMessage(result, failure, err); ok {
		p.notifySystem(title, body)
	}
}

// shortcutOutcomeMessage is the notification text of a run that reports something, and false for a silent run. A result
// that wants the palette (a view, a follow-up) is logged, since a command that runs in place should not ask for one.
func shortcutOutcomeMessage(result api.Result, failure *api.Failure, err error) (string, bool) {
	switch {
	case failure != nil:
		return failure.Message, true
	case err != nil:
		return err.Error(), true
	case result.Toast != nil:
		return result.Toast.Message, true
	case result.View != nil || result.Next != nil:
		logx.Default().Warn("shortcut result needs the palette and was not shown", logx.F{"reason": "view or follow-up"})
	}
	return "", false
}
