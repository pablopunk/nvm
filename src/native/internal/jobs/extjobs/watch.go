package extjobs

import (
	"context"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// syncWatches makes the job's folder watches match its files.changed triggers. A job whose watches did not change keeps
// them running.
func (a *Adapter) syncWatches(jobID string, watches []fileWatch) {
	signature := signatureOf(watches)
	a.mu.Lock()
	defer a.mu.Unlock()
	current, running := a.watches[jobID]
	if running && current.signature == signature {
		return
	}
	if running {
		current.cancel()
		delete(a.watches, jobID)
	}
	if len(watches) == 0 {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.watches[jobID] = watchSet{signature: signature, cancel: cancel}
	for _, watch := range watches {
		for _, root := range watch.roots {
			a.watchRoot(ctx, watch, root)
		}
	}
}

// stopWatches cancels the watches of a job that was removed.
func (a *Adapter) stopWatches(jobID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if current, running := a.watches[jobID]; running {
		current.cancel()
		delete(a.watches, jobID)
	}
}

// watchRoot starts one folder watch and forwards its changes to the scheduler as the trigger's event. The watch ends
// when ctx is cancelled, which closes its channel. A root that cannot be watched is logged and skipped.
func (a *Adapter) watchRoot(ctx context.Context, watch fileWatch, root string) {
	events, err := a.cfg.Files.Watch(ctx, root)
	if err != nil {
		a.logWarn("extension.trigger.watch.failed", watch.event)
		return
	}
	a.safegoGo("extjobs.watch", func() {
		for event := range events {
			a.forwardFileEvent(watch, root, event)
		}
	})
}

// forwardFileEvent emits the trigger's event for a change that passes the filter. An overflow means the watch lost
// changes, so the whole root counts as changed.
func (a *Adapter) forwardFileEvent(watch fileWatch, root string, event platform.Event) {
	path := event.Path
	if event.Op == platform.EventOverflow {
		path = root
	} else if !watch.filter.matches(path) {
		return
	}
	if path == "" {
		return
	}
	a.cfg.Registry.Emit(watch.event, jobs.Payload{
		jobs.ChangedPaths: []string{path},
		"event":           "files.changed",
		"trigger":         watch.trigger,
	})
}

// signatureOf identifies a set of watches by everything that decides what they watch and emit.
func signatureOf(watches []fileWatch) string {
	var b strings.Builder
	for _, watch := range watches {
		fmt.Fprintf(&b, "%s|%v|%+v|%+v;", watch.event, watch.roots, watch.filter, watch.trigger)
	}
	return b.String()
}
