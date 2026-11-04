package app

import (
	"context"
	"log"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/update"
	"github.com/pablopunk/nvm/native/internal/update/mygoengine"
)

// attachUpdater builds the update service over MyGo's signed updater and starts its schedule. A build that MyGo was not
// stamped for reports unsupported and never contacts a feed. Every change of state refreshes the menu bar and the
// palette on the UI thread.
func (p *App) attachUpdater() {
	p.updates = update.NewService(update.Options{
		Engine:   mygoengine.New(),
		Clock:    p.ports.Clock,
		Channels: newUpdateChannels(p),
		Current:  mygo.App.Version(),
		OnChange: func(state update.State) {
			log.Printf("update state: %s %q", state.Status, update.Message(state))
			p.onUI(p.refreshUpdateSurfaces)
		},
	})
	p.updater = p.updates
	safego.Go("updates.schedule", func(ctx context.Context) { p.updates.Run(ctx) })
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

// restartToUpdate restarts the app into the downloaded update, or says why it cannot.
func (p *App) restartToUpdate() error {
	if p.updates == nil {
		return update.ErrNotReady
	}
	return p.updates.RestartToUpdate()
}

// refreshUpdateSurfaces redraws what shows the update: the menu bar's update row and the open palette.
func (p *App) refreshUpdateSurfaces() {
	if p.menubar != nil {
		p.menubar.SetMenu(menubar.RowsWithUpdate(p.menubarActions(), p.onUI, p.menubarUpdateRow()))
	}
	p.Refresh()
}
