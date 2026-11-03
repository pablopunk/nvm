package files

import (
	"context"
	"slices"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// scanOutcome is what one full scan hands back to the loop.
type scanOutcome struct {
	result  scanResult
	missing []string
	err     error
}

// pendingPaths collects the paths that file system events named since the last flush. The paths are re-read when
// they are applied, so duplicates and events the OS delivered out of order do no harm. full asks for a whole scan.
type pendingPaths struct {
	paths map[string]struct{}
	full  bool
}

// newPendingPaths returns an empty set.
func newPendingPaths() pendingPaths {
	return pendingPaths{paths: map[string]struct{}{}}
}

// add records a path to re-read.
func (p *pendingPaths) add(path string) {
	p.paths[path] = struct{}{}
}

// take returns the recorded paths in sorted order and empties the set.
func (p *pendingPaths) take() []string {
	out := make([]string, 0, len(p.paths))
	for path := range p.paths {
		out = append(out, path)
	}
	slices.Sort(out)
	p.paths = map[string]struct{}{}
	return out
}

// indexLoop is the state of the index loop. Only the loop goroutine touches it.
type indexLoop struct {
	ix       *Index
	ctx      context.Context
	pending  pendingPaths
	scanning bool
	queued   bool
	debounce <-chan time.Time
	scans    chan scanOutcome
	ticker   platform.Ticker
	watches  map[string]context.CancelFunc
	missing  []string
	// degraded holds the folders whose watch overflowed; they are only reconciled.
	degraded map[string]bool
}

// run is the index loop. It publishes every snapshot, so readers only ever see a complete one. It returns when ctx ends.
func (ix *Index) run(ctx context.Context) {
	loop := &indexLoop{
		ix:      ix,
		ctx:     ctx,
		pending: newPendingPaths(),
		scans:   make(chan scanOutcome, 1),
		watches: map[string]context.CancelFunc{},
	}
	loop.ticker = ix.cfg.Clock.NewTicker(ix.cfg.Reconcile)
	defer ix.finish()
	defer loop.ticker.Stop()
	defer loop.stopWatches()
	ix.loadPersisted()
	loop.startScan()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ix.events:
			loop.noteEvent(ev)
		case <-loop.debounce:
			loop.debounce = nil
			loop.flush()
		case out := <-loop.scans:
			loop.finishScan(out)
		case <-loop.ticker.C():
			loop.requestScan()
		case <-ix.trigger:
			loop.requestScan()
		case path := <-ix.lost:
			loop.watchLost(path)
		}
	}
}

// noteEvent records what an event names and arms the debounce window on the first one.
func (l *indexLoop) noteEvent(ev platform.Event) {
	if ev.Op == platform.EventOverflow {
		l.markDegraded(ev.Path)
		l.pending.full = true
		l.armDebounce()
		return
	}
	if ev.Op == platform.EventRename && ev.OldPath != "" {
		l.pending.add(ev.OldPath)
	}
	l.pending.add(ev.Path)
	if l.touchesMissingRoot(ev.Path) {
		l.pending.full = true
	}
	l.armDebounce()
}

// armDebounce starts the debounce window when none is running.
func (l *indexLoop) armDebounce() {
	if l.debounce == nil {
		l.debounce = l.ix.cfg.Clock.After(l.ix.cfg.Debounce)
	}
}

// touchesMissingRoot reports whether a path is, contains or lies inside a root that does not exist yet. Such an event
// may be the one that creates the root, so the next step is a full scan.
func (l *indexLoop) touchesMissingRoot(path string) bool {
	for _, missing := range l.missing {
		if IsInsideRoot(path, missing) || IsInsideRoot(missing, path) {
			return true
		}
	}
	return false
}

// flush applies the pending paths to the published snapshot. While a scan runs it does nothing: the events are applied
// after the scan commits, because the scan may already have passed the folders they name.
func (l *indexLoop) flush() {
	if l.scanning {
		return
	}
	if l.pending.full {
		l.pending = newPendingPaths()
		l.requestScan()
		return
	}
	paths := l.pending.take()
	if len(paths) == 0 {
		return
	}
	d := l.changesFor(paths)
	if d.empty() {
		return
	}
	old := l.ix.Snapshot()
	l.ix.publish(d.apply(old, old.generation+1, l.ix.configKey), false)
}

// requestScan starts a full scan now, or queues one for when the running scan commits.
func (l *indexLoop) requestScan() {
	if l.scanning {
		l.queued = true
		return
	}
	l.startScan()
}

// startScan registers the watches first, so an event that arrives during the scan is kept, and then walks the roots
// in a background goroutine.
func (l *indexLoop) startScan() {
	l.scanning = true
	l.pending.full = false
	l.syncWatches()
	ix := l.ix
	ix.counters.dirs.Store(0)
	ix.counters.files.Store(0)
	ix.updateStatus(func(st *Status) { st.Scanning = true })
	ix.scanners.Add(1)
	safego.Go("files.scan", func(c context.Context) {
		defer ix.scanners.Done()
		out := ix.scanRoots(c)
		select {
		case l.scans <- out:
		case <-c.Done():
		}
	}, safego.WithContext(l.ctx))
}

// scanRoots walks every existing root. Roots that do not exist are reported as missing.
func (ix *Index) scanRoots(ctx context.Context) scanOutcome {
	existing, missing := PartitionRoots(ix.cfg.FS, ix.roots)
	starts := make([]scanStart, 0, len(existing))
	for _, root := range existing {
		starts = append(starts, scanStart{dir: root, remaining: ix.cfg.MaxDepth})
	}
	result, err := scanTrees(ctx, ix.cfg.FS, ix.scanConfig(true), starts, &ix.counters)
	return scanOutcome{result: result, missing: missing, err: err}
}

// finishScan commits a finished scan, then applies the events that arrived while it ran and starts a queued scan.
func (l *indexLoop) finishScan(out scanOutcome) {
	l.scanning = false
	if out.err != nil {
		return
	}
	l.ix.commitScan(out)
	l.missing = out.missing
	l.flush()
	if l.scanning {
		// The flush started a scan that begins after this commit, so it already covers any queued request.
		l.queued = false
		return
	}
	if l.queued {
		l.queued = false
		l.startScan()
	}
}

// finish runs when the loop returns: it waits for background saves and writes the latest snapshot if it was not saved.
func (ix *Index) finish() {
	ix.saves.Wait()
	snap := ix.snap.Load()
	if ix.cfg.Persistence != nil && snap.generation > ix.savedGen.Load() {
		ix.save(snap)
	}
}
