package apps

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// Running limits, as in the 1.x running-app status: a renderer asks for at most 30 paths at once, and a
// snapshot is trusted for RunningTTL.
const (
	RunningTTL          = 2 * time.Second
	maxRequestedRunning = 30
)

// RunningOptions wires a Running service. Candidates and Clock are required. Detect is optional: nil uses the
// Processes port, and tests and macOS (whose process table the port does not expose) pass their own.
type RunningOptions struct {
	Candidates func() []App
	Clock      platform.Clock
	Processes  platform.Processes
	Files      platform.FS
	Detect     func(candidates []App) (map[string]bool, error)
	// Changed is called after a refresh that differs from the previous snapshot.
	Changed func()
	// Platform decides how paths are compared: macOS and Windows ignore case. Empty means runtime.GOOS.
	Platform string
}

type runningSnapshot struct {
	updatedAt time.Time
	paths     map[string]bool
}

// flight is one detection in progress; callers that ask during it wait for the same result.
type flight struct {
	done  chan struct{}
	paths map[string]bool
	err   error
}

// Running answers which listed apps have a live process. Snapshot never blocks: it answers from the last
// snapshot and starts a refresh when that snapshot is older than RunningTTL.
type Running struct {
	opts RunningOptions

	mu       sync.Mutex
	idle     *sync.Cond
	snapshot *runningSnapshot
	inFlight *flight
	entries  map[string]cachedEntry
}

type cachedEntry struct {
	mtime time.Time
	entry DesktopEntry
	ok    bool
}

// NewRunning returns a service with no snapshot. The first Snapshot call starts the first detection.
func NewRunning(opts RunningOptions) *Running {
	if opts.Platform == "" {
		opts.Platform = runtime.GOOS
	}
	r := &Running{opts: opts, entries: map[string]cachedEntry{}}
	r.idle = sync.NewCond(&r.mu)
	return r
}

// Snapshot returns the requested paths that the last snapshot shows running, in request order. Each call asks for
// at most 30 paths. A stale or missing snapshot starts a background refresh; the answer is not delayed by it.
func (r *Running) Snapshot(requested []string) []string {
	paths := requestedPaths(requested)
	r.ScheduleRefresh("renderer-request")
	r.mu.Lock()
	defer r.mu.Unlock()
	var running []string
	for _, p := range paths {
		if r.snapshot != nil && r.snapshot.paths[r.normalize(p)] {
			running = append(running, p)
		}
	}
	return running
}

// ScheduleRefresh starts a background refresh unless the snapshot is fresh or a refresh is running. The refresh
// claims the in-flight slot before this returns, so two calls in a row start one detection.
func (r *Running) ScheduleRefresh(_ string) {
	r.mu.Lock()
	fresh := r.snapshot != nil && r.opts.Clock.Now().Sub(r.snapshot.updatedAt) < RunningTTL
	if fresh || r.inFlight != nil {
		r.mu.Unlock()
		return
	}
	f := r.claimLocked()
	r.mu.Unlock()
	safego.Go("apps.running", func(context.Context) { r.runFlight(f) })
}

// Invalidate drops the snapshot, for example after the app list changed.
func (r *Running) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshot = nil
}

// Refresh runs a detection now and blocks until it is done. A call during a running detection waits for that one
// instead of starting another. The result is the set of running paths in the platform's comparison form.
func (r *Running) Refresh(_ string) (map[string]bool, error) {
	r.mu.Lock()
	if f := r.inFlight; f != nil {
		r.mu.Unlock()
		<-f.done
		return f.paths, f.err
	}
	f := r.claimLocked()
	r.mu.Unlock()
	r.runFlight(f)
	return f.paths, f.err
}

// claimLocked makes a new flight the in-flight one. The caller holds r.mu and then calls runFlight.
func (r *Running) claimLocked() *flight {
	f := &flight{done: make(chan struct{})}
	r.inFlight = f
	return f
}

// runFlight detects the running apps, stores the snapshot, releases the waiters and reports a change.
func (r *Running) runFlight(f *flight) {
	r.mu.Lock()
	previous := r.snapshot
	r.mu.Unlock()

	paths, err := r.detect()
	normalized := make(map[string]bool, len(paths))
	for p := range paths {
		if key := r.normalize(p); key != "" {
			normalized[key] = true
		}
	}

	r.mu.Lock()
	f.paths, f.err = normalized, err
	if err == nil {
		r.snapshot = &runningSnapshot{updatedAt: r.opts.Clock.Now(), paths: normalized}
	}
	r.inFlight = nil
	r.idle.Broadcast()
	r.mu.Unlock()
	close(f.done)

	if err == nil && r.opts.Changed != nil && !sameSet(previous, normalized) {
		r.opts.Changed()
	}
}

// RunningAmong returns the apps of list that the current snapshot shows running, keyed by their own path. Unlike
// Snapshot it has no request cap, so the Force Quit view can list every running app. It never blocks.
func (r *Running) RunningAmong(list []App) map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]bool{}
	if r.snapshot == nil {
		return out
	}
	for _, app := range list {
		if r.snapshot.paths[r.normalize(app.Path)] {
			out[app.Path] = true
		}
	}
	return out
}

// WaitIdle blocks until no detection is running. Tests use it; the palette does not wait.
func (r *Running) WaitIdle() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for r.inFlight != nil {
		r.idle.Wait()
	}
}

func (r *Running) detect() (map[string]bool, error) {
	if r.opts.Detect != nil {
		return r.opts.Detect(r.opts.Candidates())
	}
	return r.detectWithProcesses(r.opts.Candidates())
}

// detectWithProcesses matches the candidates against the process table: on Linux a desktop entry's program or window
// class against each process's name. Other systems report ErrUnsupported through the port, and so does this.
func (r *Running) detectWithProcesses(candidates []App) (map[string]bool, error) {
	if r.opts.Processes == nil {
		return nil, platform.ErrUnsupported
	}
	list, err := r.opts.Processes.List()
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(list))
	for _, process := range list {
		names[strings.ToLower(process.Name)] = true
	}
	running := map[string]bool{}
	for _, app := range candidates {
		if entry, ok := r.desktopEntry(app.Path); ok && entry.MatchesProcessNames(names) {
			running[app.Path] = true
		}
	}
	return running, nil
}

// desktopEntry reads and parses a .desktop candidate, reusing the parse while its modification time is unchanged.
func (r *Running) desktopEntry(appPath string) (DesktopEntry, bool) {
	if r.opts.Files == nil || filepath.Ext(appPath) != ".desktop" {
		return DesktopEntry{}, false
	}
	info, err := r.opts.Files.Stat(appPath)
	if err != nil {
		return DesktopEntry{}, false
	}
	r.mu.Lock()
	cached, known := r.entries[appPath]
	r.mu.Unlock()
	if known && cached.mtime.Equal(info.ModTime()) {
		return cached.entry, cached.ok
	}
	data, err := r.opts.Files.ReadFile(appPath)
	if err != nil {
		return DesktopEntry{}, false
	}
	entry, ok := ParseDesktopEntry(string(data))
	r.mu.Lock()
	r.entries[appPath] = cachedEntry{mtime: info.ModTime(), entry: entry, ok: ok}
	r.mu.Unlock()
	return entry, ok
}

// normalize is the comparison form of a path: lowercased where the file system ignores case.
func (r *Running) normalize(p string) string {
	text := strings.TrimSpace(p)
	if r.opts.Platform == "darwin" || r.opts.Platform == "windows" {
		return strings.ToLower(text)
	}
	return text
}

func requestedPaths(requested []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range requested {
		text := strings.TrimSpace(p)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
		if len(out) == maxRequestedRunning {
			break
		}
	}
	return out
}

func sameSet(a *runningSnapshot, b map[string]bool) bool {
	if a == nil {
		return false
	}
	if len(a.paths) != len(b) {
		return false
	}
	for p := range a.paths {
		if !b[p] {
			return false
		}
	}
	return true
}
