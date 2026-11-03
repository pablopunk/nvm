package files

import (
	"context"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// syncWatches makes the set of watches match the roots: each existing root is watched, and a missing root is watched
// through its closest existing ancestor, so the event that creates it arrives. Watches no longer wanted are cancelled.
func (l *indexLoop) syncWatches() {
	wanted := map[string]bool{}
	for _, root := range l.ix.roots {
		if target := l.watchTargetFor(root); target != "" {
			wanted[target] = true
		}
	}
	for target, cancel := range l.watches {
		if !wanted[target] {
			cancel()
			delete(l.watches, target)
		}
	}
	for target := range wanted {
		if _, ok := l.watches[target]; !ok {
			l.watch(target)
		}
	}
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
