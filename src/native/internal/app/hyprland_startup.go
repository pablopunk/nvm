package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/safego"
)

const (
	// compositorListingsPoll is how often startup looks again for extension command listings.
	compositorListingsPoll = 200 * time.Millisecond
	// compositorListingsLimit is how long startup waits for the listings before it syncs with what it has. An
	// extension still listing keeps its shortcuts bound meanwhile, and a later listing requests another sync.
	compositorListingsLimit = 10 * time.Second
)

// startCompositorSync brings the Hyprland module in line with the shortcuts once per start. The shortcut table binds
// before the extensions load, and a request made then is dropped, so a change made while Nevermind was not running
// (settings sync, a hand edit) never reached the module. It runs after the extensions have loaded and listed their
// commands, because a bind is written only for an action that can run.
func (p *App) startCompositorSync() {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return
	}
	safego.Go("hyprland.startup-sync", func(ctx context.Context) {
		awaitStartupSync(ctx, startupSync{
			settled: p.exts.settled.wait(), listingsKnown: p.extensionListingsKnown,
			poll: compositorListingsPoll, limit: compositorListingsLimit, request: p.startCompositorSyncing,
		})
	})
	p.watchSharedConfig()
}

// startupSync is what the startup sync waits for and the request it makes.
type startupSync struct {
	settled       <-chan struct{}
	listingsKnown func() bool
	poll, limit   time.Duration
	request       func()
}

// awaitStartupSync waits for the extensions to load and list their commands, but never longer than limit, then
// requests one sync. It returns without a request when ctx ends first.
func awaitStartupSync(ctx context.Context, w startupSync) {
	select {
	case <-w.settled:
	case <-ctx.Done():
		return
	}
	deadline := time.NewTimer(w.limit)
	defer deadline.Stop()
	tick := time.NewTicker(w.poll)
	defer tick.Stop()
	for !w.listingsKnown() {
		select {
		case <-tick.C:
		case <-deadline.C:
			w.request()
			return
		case <-ctx.Done():
			return
		}
	}
	w.request()
}

// extensionListingsKnown reports whether every extension that may run has listed its commands.
func (p *App) extensionListingsKnown() bool {
	m := p.extensionManager()
	if m == nil {
		return false
	}
	m.CachedCommands() // starts discovery for any extension that has not listed yet
	for _, listing := range m.List() {
		if p.extensionAwaitingCommands(listing.ID) {
			return false
		}
	}
	return true
}

// startCompositorSyncing lets shortcut changes sync from now on and syncs once for the shortcuts as they are.
func (p *App) startCompositorSyncing() {
	p.compositorReady.Store(true)
	p.requestCompositorSync()
}

// watchSharedConfig follows config.json while the app runs: it is shared with other machines, so its shortcuts can
// change under the app. A change rebinds the shortcuts and syncs the module.
func (p *App) watchSharedConfig() {
	if p.ports.FS == nil {
		return
	}
	path := sharedConfigPath()
	safego.Go("hyprland.config-watch", func(ctx context.Context) {
		events, err := p.ports.FS.Watch(ctx, filepath.Dir(path))
		if err != nil {
			return
		}
		for event := range events {
			if filepath.Base(event.Path) == filepath.Base(path) {
				p.configRefresh.request(p.onUI, p.followSharedConfig)
			}
		}
	})
}

// followSharedConfig binds the shortcuts the file now holds; the document announces a change itself only when the app
// saves, so this asks for the sync directly. The sync writes nothing when the module is already right.
func (p *App) followSharedConfig() {
	if p.shortcutMgr != nil {
		p.shortcutMgr.Refresh()
	}
	p.requestCompositorSync()
}
