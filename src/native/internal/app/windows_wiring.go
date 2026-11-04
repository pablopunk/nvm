package app

import (
	"context"
	"os"

	"github.com/pablopunk/nvm/native/internal/palette/windows"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// windowSettingsFile is the document the window management settings persist in.
const windowSettingsFile = "window-management.json"

// attachWindows builds the window management engine over the ports and loads its settings. Building it makes no
// window system call: those start with the first window command or view, so the first paint does not wait for them.
func (p *App) attachWindows() {
	var doc *store.Collection[windowmgmt.SettingsDocument]
	if p.store != nil {
		doc = store.Doc(p.store, windowSettingsFile, 1, windowmgmt.DefaultSettingsDocument)
	}
	p.windowSettings = windowmgmt.NewSettingsStore(doc)
	p.windows = windowmgmt.NewService(windowmgmt.Deps{
		Windows: p.ports.Windows,
		Focus:   p,
		Clock:   p.ports.Clock,
		SelfPID: os.Getpid(),
	})
}

// windowCommands are the window management entries of the command list. They exist only where the system can
// manage other apps' windows; elsewhere the commands are omitted rather than shown disabled.
func (p *App) windowCommands() []command {
	if !p.windowManagementEnabled() {
		return nil
	}
	entries := windows.Entries(p.windowDeps())
	commands := make([]command, 0, len(entries))
	for _, entry := range entries {
		commands = append(commands, command{
			ID: entry.ID, Title: entry.Title, Subtitle: entry.Subtitle, Icon: entry.Icon,
			Keywords: entry.Keywords, Hidden: entry.Hidden, Run: entry.Run, PhraseMatch: true,
		})
	}
	return commands
}

// windowManagementEnabled reports whether the system can manage other apps' windows.
func (p *App) windowManagementEnabled() bool {
	return p.ports.Capabilities.Supported(platform.CapabilityWindowManagement)
}

func (p *App) windowDeps() windows.Deps {
	return windows.Deps{
		Engine:   p.windows,
		Settings: p.windowSettings,
		Target:   p.windowTarget,
		OnUI:     p.onUI,
		Notify:   p.notifyWindowOutcome,
		Go:       func(name string, fn func(context.Context)) { safego.Go(name, fn) },
	}
}

// notifyWindowOutcome shows a window command's failure as a system notification, because the command runs after
// the palette has hidden to hand focus back.
func (p *App) notifyWindowOutcome(title, body string) {
	p.notifySystem(title, body)
}
