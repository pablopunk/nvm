package app

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/update"
)

// attachUpdater builds the update service over MyGo's signed updater and starts its schedule. A build that MyGo was not
// stamped for reports unsupported and never contacts a feed. Every change of state refreshes the menu bar and the
// palette on the UI thread.
func (p *App) attachUpdater() {
	p.updatesCtx, p.stopUpdates = context.WithCancel(context.Background())
	p.updates = update.NewService(update.Options{
		Engine:   real.NewSelfUpdater(),
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

// checkForUpdatesWithCompletion reports the updater result on the UI thread and completes the command after Check returns.
func (p *App) checkForUpdatesWithCompletion(ctx context.Context, report func(string), complete func(analytics.OperationOutcome)) {
	var once sync.Once
	finish := func(outcome analytics.OperationOutcome) { once.Do(func() { complete(outcome) }) }
	safego.Go("updates.check", func(taskCtx context.Context) {
		ctx, cancel := context.WithCancel(ctx)
		stopTask := context.AfterFunc(taskCtx, cancel)
		stopApp := context.AfterFunc(p.updatesCtx, cancel)
		defer func() { stopTask(); stopApp(); cancel() }()
		var outcome analytics.OperationOutcome
		defer func() {
			if recovered := recover(); recovered != nil {
				finish(analytics.OutcomeFailed)
				panic(recovered)
			}
			if outcome != "" {
				finish(outcome)
			}
		}()
		state, owned := p.updates.CheckOwned(ctx)
		if !owned {
			return
		}
		outcome = updateCheckOutcome(ctx.Err(), state)
		p.onUI(func() { report(update.Message(state)) })
	})
}

// stopUpdateChecks cancels app-owned check workers when the application begins quitting.
func (p *App) stopUpdateChecks(_ *mygo.QuitEvent) {
	if p.stopUpdates != nil {
		p.stopUpdates()
	}
}

func updateCheckOutcome(ctxErr error, state update.State) analytics.OperationOutcome {
	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return analytics.OutcomeTimedOut
	case errors.Is(ctxErr, context.Canceled):
		return analytics.OutcomeCancelled
	case state.Status == update.StatusFailed && state.Stage == update.StageCheck:
		return analytics.OutcomeFailed
	default:
		return analytics.OutcomeSuccess
	}
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
	p.refreshMenubarMenu()
	p.Refresh()
}
