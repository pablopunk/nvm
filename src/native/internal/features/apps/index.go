package apps

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// RefreshDebounce is the least time between two scans the palette asks for. A request inside the window is
// dropped; a request during a scan is answered by one more scan after it.
const RefreshDebounce = time.Second

// IndexHooks are the notifications the index sends to the code that shows apps. Each is called off the UI
// thread; the caller hands the work to the UI thread itself.
type IndexHooks struct {
	// InvalidateRunning drops the running snapshot, which no longer matches the list.
	InvalidateRunning func()
	// ScheduleRunningRefresh asks for a fresh running snapshot for the new list.
	ScheduleRunningRefresh func(reason string)
	// Indexed reports the number of apps after every list change.
	Indexed func(count int)
	// Error reports a failed scan or save. The previous list stays in place.
	Error func(err error)
}

// IndexOptions wires an Index. Scan and Clock are required.
type IndexOptions struct {
	Scan  Scanner
	Cache Cache // nil persists nothing
	Clock platform.Clock
	// Identity maps an app's path to the key first-seen dates are kept under. Nil keeps the path as it is;
	// macOS and Windows pass a lowercasing function because their file systems ignore case.
	Identity func(path string) string
	Hooks    IndexHooks
}

// Index is the list of installed apps: scanned off the caller's thread, deduped and dated, persisted, and
// refreshed on request. Apps returns the last good list at any time, including while a scan runs.
type Index struct {
	opts IndexOptions

	// scanMu serializes scans and their saves, so a slow scan can never overwrite a newer list.
	scanMu sync.Mutex

	mu        sync.Mutex
	idle      *sync.Cond
	apps      []App
	firstSeen FirstSeen
	readOnly  bool
	inFlight  bool
	pending   bool
	lastDone  time.Time
}

// NewIndex returns an empty index. Call LoadCache to show the saved list, then RequestRefresh.
func NewIndex(opts IndexOptions) *Index {
	if opts.Identity == nil {
		opts.Identity = func(path string) string { return path }
	}
	x := &Index{opts: opts, firstSeen: FirstSeen{ByID: map[string]int64{}}}
	x.idle = sync.NewCond(&x.mu)
	return x
}

// Apps returns a copy of the current list, sorted by name.
func (x *Index) Apps() []App {
	x.mu.Lock()
	defer x.mu.Unlock()
	return slices.Clone(x.apps)
}

// LoadCache shows the saved list until the first scan lands. A snapshot from a newer build is ignored and the
// index stays read-only, so that data is never overwritten. A scan that already finished wins over the cache.
func (x *Index) LoadCache() error {
	if x.opts.Cache == nil {
		return nil
	}
	snapshot, err := x.opts.Cache.Load()
	if err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if snapshot.Version > SnapshotVersion {
		x.readOnly = true
		return nil
	}
	if x.apps == nil {
		x.apps = slices.Clone(snapshot.Apps)
	}
	if snapshot.FirstSeen.ByID != nil {
		x.firstSeen = cloneFirstSeen(snapshot.FirstSeen)
	}
	return nil
}

// Scan lists the apps now and blocks until the list is updated. On a failed scan the previous list stays and the
// error is reported through Hooks.Error and returned. Callers run it off the UI thread.
func (x *Index) Scan() error {
	x.scanMu.Lock()
	defer x.scanMu.Unlock()
	scan, err := x.opts.Scan()
	if err != nil {
		x.reportError(err)
		return err
	}
	x.adopt(scan, false)
	x.notifyIndexed()
	x.baselineIfNeeded()
	return nil
}

// baselineIfNeeded confirms the first-seen baseline with a second complete scan. Until a complete scan confirms
// it, every app is dated 0, so the first run after install does not mark the whole machine as new.
func (x *Index) baselineIfNeeded() {
	x.mu.Lock()
	needed := !x.firstSeen.Initialized
	x.mu.Unlock()
	if !needed {
		return
	}
	scan, err := x.opts.Scan()
	if err != nil {
		x.reportError(err)
		return
	}
	if !scan.Complete {
		return
	}
	x.adopt(scan, true)
	x.notifyIndexed()
}

// adopt dates and dedupes a scan into the index, then saves the snapshot when it changed.
func (x *Index) adopt(scan ScanResult, initializeBaseline bool) {
	x.mu.Lock()
	tracked, next, _ := TrackFirstSeen(scan.Apps, x.firstSeen, initializeBaseline, x.opts.Clock.Now().UnixMilli(), x.opts.Identity)
	x.apps = DedupeAndSort(tracked)
	x.firstSeen = next
	snapshot := Snapshot{Version: SnapshotVersion, Apps: slices.Clone(x.apps), FirstSeen: cloneFirstSeen(next)}
	save := !x.readOnly && x.opts.Cache != nil
	x.mu.Unlock()
	if save {
		if err := x.opts.Cache.Save(snapshot); err != nil {
			x.reportError(err)
		}
	}
}

func (x *Index) notifyIndexed() {
	x.mu.Lock()
	count := len(x.apps)
	x.mu.Unlock()
	if x.opts.Hooks.InvalidateRunning != nil {
		x.opts.Hooks.InvalidateRunning()
	}
	if x.opts.Hooks.Indexed != nil {
		x.opts.Hooks.Indexed(count)
	}
	if x.opts.Hooks.ScheduleRunningRefresh != nil {
		x.opts.Hooks.ScheduleRunningRefresh("apps-indexed")
	}
}

func (x *Index) reportError(err error) {
	if x.opts.Hooks.Error != nil {
		x.opts.Hooks.Error(err)
	}
}

// RequestRefresh starts a scan in the background unless one is running (then one more runs after it) or the
// last scan finished less than RefreshDebounce ago. It never blocks.
func (x *Index) RequestRefresh() {
	x.mu.Lock()
	if x.inFlight {
		x.pending = true
		x.mu.Unlock()
		return
	}
	if !x.lastDone.IsZero() && x.opts.Clock.Now().Sub(x.lastDone) < RefreshDebounce {
		x.mu.Unlock()
		return
	}
	x.inFlight = true
	x.mu.Unlock()
	safego.Go("apps.index", x.drain)
}

// drain runs scans until no request is pending, then clears the in-flight flag.
func (x *Index) drain(ctx context.Context) {
	for {
		_ = x.Scan()
		x.mu.Lock()
		x.lastDone = x.opts.Clock.Now()
		again := x.pending && ctx.Err() == nil
		x.pending = false
		if !again {
			x.inFlight = false
			x.idle.Broadcast()
			x.mu.Unlock()
			return
		}
		x.mu.Unlock()
	}
}

// WaitIdle blocks until no background scan is running. The palette does not need it; tests and shutdown use it.
func (x *Index) WaitIdle() {
	x.mu.Lock()
	defer x.mu.Unlock()
	for x.inFlight {
		x.idle.Wait()
	}
}
