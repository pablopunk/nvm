package files

import (
	"context"
	"errors"
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
// A folder whose watch overflowed, or that fits no entry limit, is not watched again: the reconcile scan covers it.
func (l *indexLoop) syncWatches() {
	wanted := map[string]bool{}
	for _, root := range l.ix.roots {
		if target := l.watchTargetFor(root); target != "" && !l.degraded[target] && !l.unwatchable[target] {
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
		l.ix.log.Info("files.watch.degraded", logx.F{"reason": "entry limit"})
	}
	l.degraded[target] = true
}

// markUnwatchable records that no watch fits target under the entry limit. Like a degraded folder it is only
// reconciled, and it is logged once, when it is marked.
func (l *indexLoop) markUnwatchable(target string) {
	if l.unwatchable == nil {
		l.unwatchable = map[string]bool{}
	}
	l.unwatchable[target] = true
	l.ix.log.Info("files.watch.unwatched", logx.F{"root": DisplayPath(l.ix.cfg.Home, target), "reason": "entry limit"})
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

// watch starts the watch of one target: a root is watched at the deepest level its entry limit allows, and a folder
// above missing roots only down to them.
func (l *indexLoop) watch(target string) {
	if slices.Contains(l.ix.roots, target) {
		l.watchRoot(target)
		return
	}
	l.watchAncestor(target)
}

// watchRoot watches a root as deep as the scan reads, stepping to shallower levels while the platform's entry limit
// refuses the tree. Each level it settles on is remembered, so a restart begins there and the step is logged once. The
// folders below the watched level are covered by the reconcile scan, as 1.x does on Linux when it cannot watch every
// folder. A root that fits at no level is reconciled alone.
func (l *indexLoop) watchRoot(root string) {
	ix := l.ix
	start := l.startLevel(root)
	for depth := start; depth >= 0; depth-- {
		err := l.startWatch(root, ix.watchSkipAt(depth))
		if err == nil {
			l.rememberLevel(root, depth)
			if depth < start {
				ix.log.Info("files.watch.narrowed", logx.F{"root": DisplayPath(ix.cfg.Home, root), "depth": depth, "maxDepth": ix.cfg.MaxDepth})
			}
			return
		}
		if !errors.Is(err, platform.ErrWatchLimit) {
			ix.log.Warn("files.watch.failed", logx.F{"reason": err.Error()})
			return
		}
	}
	l.markUnwatchable(root)
}

// watchAncestor watches the closest existing folder above missing roots. Only the folders on the way to each missing
// root are kept, so the creation of the next folder on that path is seen without watching the rest of the ancestor.
func (l *indexLoop) watchAncestor(target string) {
	err := l.startWatch(target, skipOffPaths(l.missingRootPathsUnder(target)))
	switch {
	case err == nil:
	case errors.Is(err, platform.ErrWatchLimit):
		l.markUnwatchable(target)
	default:
		l.ix.log.Warn("files.watch.failed", logx.F{"reason": err.Error()})
	}
}

// startLevel returns the depth a root's watch is first tried at: the level it last settled on, or the scan's depth.
func (l *indexLoop) startLevel(root string) int {
	if depth, ok := l.levels[root]; ok {
		return depth
	}
	return l.ix.cfg.MaxDepth
}

// rememberLevel records the level a root's watch settled on.
func (l *indexLoop) rememberLevel(root string, depth int) {
	if l.levels == nil {
		l.levels = map[string]int{}
	}
	l.levels[root] = depth
}

// missingRootPathsUnder returns the missing roots watched through target, as paths relative to it.
func (l *indexLoop) missingRootPathsUnder(target string) []string {
	var rels []string
	for _, root := range l.ix.roots {
		if root != target && l.watchTargetFor(root) == target {
			rels = append(rels, relativeTo(target, root))
		}
	}
	return rels
}

// startWatch starts one watch of target without the entries skip drops, and a goroutine that forwards its events to
// the loop. The error is returned as is; the caller decides how it is logged.
func (l *indexLoop) startWatch(target string, skip func(rel string, isDir bool) bool) error {
	ix := l.ix
	watchCtx, cancel := context.WithCancel(l.ctx)
	events, err := ix.cfg.FS.WatchFiltered(watchCtx, target, skip)
	if err != nil {
		cancel()
		return err
	}
	l.watches[target] = cancel
	ix.watchers.Add(1)
	safego.Go("files.watch", func(c context.Context) {
		defer ix.watchers.Done()
		ix.forward(c, target, events)
	}, safego.WithContext(watchCtx))
	return nil
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
