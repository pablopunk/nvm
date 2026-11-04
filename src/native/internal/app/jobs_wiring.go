package app

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/jobs/extjobs"
	"github.com/pablopunk/nvm/native/internal/jobs/hostjobs"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/background"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// startJobs builds the background scheduler, registers the host jobs and the sync that keeps the extension triggers
// in step with the running extensions. Its state is saved in the user store when this instance holds the store, and
// kept in memory otherwise. Quitting stops every trigger.
func (p *App) startJobs() {
	cfg := jobs.Config{Clock: p.ports.Clock, Log: logx.Default()}
	if p.userStore != nil && !p.userReadOnly {
		cfg.State = jobs.StateDocument(p.userStore)
	}
	sched, err := jobs.New(cfg)
	if err != nil {
		log.Println("jobs:", err)
		return
	}
	p.background = sched
	if p.clip != nil {
		p.registerHostJob(hostjobs.ClipboardImageCleanup(p.clip.images, p.clip.history))
	}
	home, _ := os.UserHomeDir()
	triggers := extjobs.New(context.Background(), extjobs.Config{
		Host:     p.extensionTriggerHost,
		Registry: sched,
		Files:    p.ports.FS,
		Home:     home,
	})
	p.registerHostJob(triggers.SyncJob())
	stopForwarding := p.forwardClipboardChanges()
	mygo.App.OnQuit(func() {
		stopForwarding()
		triggers.Close()
		_ = sched.Close()
	})
}

// registerHostJob adds one host job and logs a definition the scheduler refuses, which is a programming error.
func (p *App) registerHostJob(def jobs.Definition) {
	if err := p.background.Register(def); err != nil {
		log.Println("jobs:", err)
	}
}

// extensionTriggerHost is the extension manager as the trigger sync reads it. It is nil while extensions load.
func (p *App) extensionTriggerHost() extjobs.Host {
	if m := p.extensionManager(); m != nil {
		return m
	}
	return nil
}

// extensionsChanged asks the trigger sync to run now, after an extension was enabled, disabled, removed or reloaded.
func (p *App) extensionsChanged() {
	if p.background != nil {
		p.background.Emit(extjobs.SyncEvent, nil)
	}
}

// forwardClipboardChanges emits clipboard.changed when the newest history entry changes, so a clipboard trigger
// fires once per copy. It returns the function that stops forwarding.
func (p *App) forwardClipboardChanges() func() {
	if p.clip == nil {
		return func() {}
	}
	history := p.clip.history
	changes, cancel := history.Subscribe()
	stop := make(chan struct{})
	last := newestClipboardID(history)
	safego.Go("jobs.clipboard.forward", func(context.Context) {
		for {
			select {
			case <-stop:
				return
			case <-changes:
				if id := newestClipboardID(history); id != last {
					last = id
					p.background.Emit(extjobs.ClipboardChangedEvent, nil)
				}
			}
		}
	})
	return func() {
		cancel()
		close(stop)
	}
}

// newestClipboardID is the ID of the first history row, or "" when the history is empty.
func newestClipboardID(history *clipboard.History) string {
	entries := history.Find(clipboard.Query{Limit: 1})
	if len(entries) == 0 {
		return ""
	}
	return entries[0].ID
}

// backgroundTasksView is the Background Tasks page over the scheduler.
func (p *App) backgroundTasksView() *palette.View {
	return background.View(p.backgroundSource(), time.Now)
}

// backgroundSource is the scheduler as the page reads it, or nil before the scheduler starts.
func (p *App) backgroundSource() background.Source {
	if p.background == nil {
		return nil
	}
	return p.background
}
