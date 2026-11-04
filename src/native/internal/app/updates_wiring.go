package app

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/update"
)

// attachUpdater gives the app its updater. Builds that cannot update themselves yet get update.NoUpdates, which
// contacts nothing; WP5.2 replaces it with the release channel.
func (p *App) attachUpdater() {
	p.updater = update.NoUpdates{Version: nativeClientVersion}
}

// checkForUpdates asks the updater off the UI thread and passes the answer to report on the UI thread.
func (p *App) checkForUpdates(report func(msg string)) {
	updater := p.updater
	safego.Go("updates.check", func(ctx context.Context) {
		state := updater.Check(ctx)
		p.onUI(func() { report(update.Message(state)) })
	})
}

// notifyUpdate shows a check's answer as a desktop notification, for checks started from the menu bar while the
// palette is hidden.
func (p *App) notifyUpdate(msg string) {
	_ = p.ports.Notifier.Show("Nevermind", msg)
}
