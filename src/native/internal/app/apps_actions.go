package app

import (
	"context"
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Long app work runs off the UI thread: quitting waits out grace periods and uninstall moves files. The outcome comes
// back to the UI thread as a toast or a result view.

// quitApps asks each app to quit, or force quits it, then reports the outcome on the UI thread.
func (p *App) quitApps(list []apps.App, force bool) {
	if len(list) == 0 {
		return
	}
	safego.Go("apps.quit", func(context.Context) {
		messages := make([]string, 0, len(list))
		for _, a := range list {
			messages = append(messages, p.quitOne(a, force))
		}
		if p.panel != nil {
			p.panel.Update(func() {
				p.Toast(strings.Join(messages, " "))
				p.ReloadRows()
			})
		}
	})
}

// quitOne quits one app and describes the result in one sentence.
func (p *App) quitOne(a apps.App, force bool) string {
	names := p.appsvc.ProcessNames(a)
	var (
		outcome apps.QuitOutcome
		err     error
	)
	if force {
		outcome, err = p.appsvc.Quitter.ForceQuit(names)
	} else {
		outcome, err = p.appsvc.Quitter.Quit(names)
	}
	return describeQuit(a.Name, force, outcome, err)
}

// describeQuit is the sentence the toast shows after a quit. It never claims more than the outcome shows.
func describeQuit(name string, force bool, outcome apps.QuitOutcome, err error) string {
	switch {
	case errors.Is(err, platform.ErrUnsupported):
		return name + " cannot be quit on this system yet."
	case err != nil:
		return "Could not quit " + name + ": " + err.Error()
	case outcome.Found == 0:
		return name + " is not running."
	case outcome.Failed > 0 && outcome.Killed == 0 && outcome.Terminated == 0:
		return "Could not quit " + name + "."
	case force:
		return "Force quit " + name + "."
	case outcome.Killed > 0:
		return name + " did not quit in time and was force quit."
	default:
		return name + " quit."
	}
}

// runUninstall moves the reviewed selection to the Trash off the UI thread, then opens the result view.
func (p *App) runUninstall(snapshot apps.UninstallSnapshot, selected map[string]bool) {
	safego.Go("apps.uninstall", func(context.Context) {
		result := p.appsvc.Uninstaller.Trash(snapshot, selected)
		view := p.uninstallResultView(result)
		if p.panel != nil {
			p.panel.Update(func() { p.Open(view) })
		}
	})
}
