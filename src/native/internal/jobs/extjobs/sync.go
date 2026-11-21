package extjobs

import (
	"context"
	"strings"
	"sync"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Adapter keeps the extension jobs in step with the running extensions. It owns the folder watches of files.changed
// triggers. Its sync job is the only writer of the jobs and watches, so runs of it never overlap.
type Adapter struct {
	cfg    Config
	log    *logx.Logger
	ctx    context.Context
	cancel context.CancelFunc

	mu               sync.Mutex
	watches          map[string]watchSet
	loginSent        bool
	frontmostPolling bool
	frontmostLast    string
}

// watchSet is the set of watches of one job, with the signature that tells when they must change.
type watchSet struct {
	signature string
	cancel    context.CancelFunc
}

// New returns an adapter. Close stops its watches; the adapter must not be used afterwards.
func New(ctx context.Context, cfg Config) *Adapter {
	ctx, cancel := context.WithCancel(ctx)
	log := cfg.Log
	if log == nil {
		log = logx.Default()
	}
	return &Adapter{cfg: cfg, log: log, ctx: ctx, cancel: cancel, watches: map[string]watchSet{}}
}

// Close stops every folder watch.
func (a *Adapter) Close() {
	a.cancel()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.watches = map[string]watchSet{}
}

// SyncJob is the host job that reconciles the extension jobs with the running extensions. It runs at startup, when an
// extension is changed (SyncEvent), and every SyncEvery.
func (a *Adapter) SyncJob() jobs.Definition {
	return jobs.Definition{
		ID:    SyncJobID,
		Title: "Extension Triggers",
		Owner: jobs.OwnerHost,
		Scope: "extensions",
		Triggers: []jobs.Trigger{
			{Kind: jobs.TriggerStartup, Delay: SyncDelay},
			{Kind: jobs.TriggerInterval, Every: SyncEvery},
			{Kind: jobs.TriggerEvent, Event: SyncEvent, Debounce: syncDebounce},
		},
		Budget: jobs.Budget{Timeout: syncTimeout},
		Run: func(ctx context.Context, _ jobs.Run) error {
			return a.reconcile(ctx)
		},
	}
}

// reconcile registers a job for every command that needs one, replaces the changed ones and removes the jobs of
// extensions that are no longer running. An extension that fails to answer keeps its jobs as they are.
func (a *Adapter) reconcile(ctx context.Context) error {
	host := a.hostOrNil()
	if host == nil {
		return jobs.Skipped{Reason: "the extensions are still loading"}
	}
	wanted := map[string]bool{}
	answering := map[string]bool{}
	loginWanted := false
	frontmostWanted := false
	for _, result := range host.Commands(ctx) {
		answering[result.ExtensionID] = true
		if result.Failure != nil {
			continue
		}
		for _, command := range result.Commands {
			p, ok := planFor(result.ExtensionID, command, a.cfg.Home)
			if !ok {
				continue
			}
			wanted[p.def.ID] = true
			loginWanted = loginWanted || declaresLogin(command)
			frontmostWanted = frontmostWanted || declaresFrontmost(command)
			a.apply(p)
		}
	}
	a.removeStale(wanted, answering)
	a.syncFrontmostPoll(frontmostWanted)
	a.emitLoginOnce(loginWanted)
	return nil
}

// hostOrNil returns the extension host, or nil while extensions are still loading.
func (a *Adapter) hostOrNil() Host {
	if a.cfg.Host == nil {
		return nil
	}
	return a.cfg.Host()
}

// apply registers one planned job and makes its watches match its triggers.
func (a *Adapter) apply(p plan) {
	p.def.Run = a.runCommand(p)
	if err := a.cfg.Registry.Register(p.def); err != nil {
		a.logWarn("extension.job.register.failed", p.def.ID)
		return
	}
	a.syncWatches(p.def.ID, p.watches)
}

// removeStale unregisters the extension jobs that no running extension still declares.
func (a *Adapter) removeStale(wanted, answering map[string]bool) {
	for _, snap := range a.cfg.Registry.Snapshot() {
		if snap.Owner != jobs.OwnerExtension || !strings.HasPrefix(snap.ID, JobPrefix) {
			continue
		}
		if wanted[snap.ID] || answering[snap.Scope] {
			continue
		}
		a.cfg.Registry.Unregister(snap.ID)
		a.stopWatches(snap.ID)
	}
}

// emitLoginOnce sends the login event once per launch, when a login trigger first exists, as the 1.x host did at ready.
func (a *Adapter) emitLoginOnce(wanted bool) {
	a.mu.Lock()
	send := wanted && !a.loginSent
	a.loginSent = a.loginSent || wanted
	a.mu.Unlock()
	if send {
		a.cfg.Registry.Emit(jobs.EventLogin, nil)
	}
}

func declaresLogin(command api.Command) bool {
	for _, trigger := range command.Triggers {
		if trigger.Type == api.TriggerLogin {
			return true
		}
	}
	return false
}

func (a *Adapter) logWarn(message, jobID string) {
	a.log.Warn(message, logx.F{"job": jobID})
}

func (a *Adapter) logInfo(message, jobID string) {
	a.log.Info(message, logx.F{"job": jobID})
}

// safegoGo runs fn under safego with the adapter's context, so a panic in a watch is reported and not fatal.
func (a *Adapter) safegoGo(name string, fn func()) {
	safego.Go(name, func(context.Context) { fn() }, safego.WithContext(a.ctx))
}
