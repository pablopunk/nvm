package files

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// maxWatchTargets caps the watches one index holds. Roots are few, so the cap is reached only by an index configured with
// many roots; the folders past it are covered by the periodic reconcile alone.
const maxWatchTargets = 32

// syncWatches makes the set of watches match the roots: each existing root is watched, and a missing root is watched
// through its closest existing ancestor, so the event that creates it arrives. Watches no longer wanted are cancelled.
// A folder whose watch overflowed is not watched again: its changes come from the periodic reconcile.
func (l *indexLoop) syncWatches() {
	wanted := map[string]bool{}
	for _, root := range l.ix.roots {
		if target := l.watchTargetFor(root); target != "" && !l.degraded[target] {
			wanted[target] = true
		}
	}
	for target, cancel := range l.watches {
		if !wanted[target] {
			cancel()
			delete(l.watches, target)
		}
	}
	for _, target := range sortedKeys(wanted) {
		if _, ok := l.watches[target]; ok {
			continue
		}
		if len(l.watches) >= maxWatchTargets {
			l.ix.log.Warn("files.watch.limit", logx.F{"maxTargets": maxWatchTargets})
			return
		}
		l.watch(target)
	}
}

// sortedKeys returns the keys of set in sorted order, so watches start in the same order every time.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// markDegraded records that the watch on target overflowed. The loop keeps the folder out of future watches, so the
// reconcile scan is the only thing that refreshes it.
func (l *indexLoop) markDegraded(target string) {
	if l.degraded == nil {
		l.degraded = map[string]bool{}
	}
	if !l.degraded[target] {
		l.ix.log.Warn("files.watch.degraded", logx.F{"reason": "entry limit"})
	}
	l.degraded[target] = true
}

// watchTargetFor returns the folder to watch for a root: the root when it exists, otherwise its closest existing
// ancestor, or "" when none exists.
func (l *indexLoop) watchTargetFor(root string) string {
	for dir := root; ; dir = filepath.Dir(dir) {
		if l.exists(dir) {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// exists reports whether a path can be described.
func (l *indexLoop) exists(path string) bool {
	_, err := l.ix.cfg.FS.Stat(path)
	return err == nil
}

// watch starts one watch and a goroutine that forwards its events to the loop. A watch that fails to start is logged;
// the reconcile scan covers what it would have reported.
func (l *indexLoop) watch(target string) {
	ix := l.ix
	watchCtx, cancel := context.WithCancel(l.ctx)
	events, err := ix.cfg.FS.Watch(watchCtx, target)
	if err != nil {
		cancel()
		ix.log.Warn("files.watch.failed", logx.F{"reason": err.Error()})
		return
	}
	l.watches[target] = cancel
	ix.watchers.Add(1)
	safego.Go("files.watch", func(c context.Context) {
		defer ix.watchers.Done()
		ix.forward(c, target, events)
	}, safego.WithContext(watchCtx))
}

// forward copies events from one watch to the loop until the watch closes. A watch that closes while its context is
// still live was lost by the OS; the loop is told so it can watch again.
func (ix *Index) forward(ctx context.Context, target string, events <-chan platform.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				if ctx.Err() == nil {
					select {
					case ix.lost <- target:
					case <-ctx.Done():
					}
				}
				return
			}
			select {
			case ix.events <- ev:
			case <-ctx.Done():
				return
			}
		}
	}
}

// watchLost forgets a watch the OS dropped and asks for a scan, which registers the watch again.
func (l *indexLoop) watchLost(target string) {
	if cancel, ok := l.watches[target]; ok {
		cancel()
		delete(l.watches, target)
	}
	l.requestScan()
}

// stopWatches cancels every watch. Their channels close and their forwarders return.
func (l *indexLoop) stopWatches() {
	for target, cancel := range l.watches {
		cancel()
		delete(l.watches, target)
	}
}
