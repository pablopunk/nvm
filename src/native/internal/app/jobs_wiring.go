package app

import (
	"context"
	"log"
	"os"

	"github.com/egoist/mygo"

	backgroundtasksbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/backgroundtasks"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/jobs/extjobs"
	"github.com/pablopunk/nvm/native/internal/jobs/hostjobs"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
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
	if p.chats != nil {
		p.registerHostJob(hostjobs.AIConversationCleanup(p.sweepAIConversations))
	}
	home, _ := os.UserHomeDir()
	triggerCfg := extjobs.Config{Host: p.extensionTriggerHost, Registry: sched, Files: p.ports.FS, Home: home}
	if p.ports.Windows != nil {
		triggerCfg.Frontmost = p.frontmostAppID
	}
	triggers := extjobs.New(context.Background(), triggerCfg)
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

// frontmostAppID names the app in front by bundle ID, then by name, or "" when no app is.
func (p *App) frontmostAppID(ctx context.Context) (string, error) {
	app, found, err := p.ports.Windows.Frontmost(ctx)
	if err != nil || !found {
		return "", err
	}
	if app.BundleID != "" {
		return app.BundleID, nil
	}
	return app.Name, nil
}

// backgroundTasksPage is the Background Tasks page the root's command opens. The Background Tasks built-in declares its
// rows, and the palette re-reads them while the page is open.
func (p *App) backgroundTasksPage() *palette.View {
	fallback := &palette.View{Title: "Background Tasks", Placeholder: "Search background tasks", EmptyTitle: "No background tasks registered.", Search: func(string) []search.Item { return nil }}
	return p.builtInOpenPage(backgroundtasksbuiltin.ID, fallback)
}
