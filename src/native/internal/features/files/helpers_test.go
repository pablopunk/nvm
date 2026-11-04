package files

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/testpath"
)

// testHome is the home folder every test uses. Tests never read the real home folder.
var testHome = testpath.Abs("home", "me")

// testDebounce is the debounce window tests configure. The fake clock advances by it to apply pending events.
const testDebounce = 100 * time.Millisecond

// waitTimeout bounds how long a test waits for a signal before it fails. It is a guard against a hang, not a sleep:
// a healthy test never reaches it.
const waitTimeout = 10 * time.Second

// seedFiles creates each path in fsys with its own name as content, creating parent folders.
func seedFiles(fsys *fake.FS, paths ...string) {
	for _, p := range paths {
		fsys.Seed(p, []byte(p))
	}
}

// newTestIndex returns an index over fsys and clock that signals changes on the returned channel. The index is closed
// when the test ends.
func newTestIndex(t *testing.T, fsys *fake.FS, clock *fake.Clock, cfg Config) (*Index, <-chan struct{}) {
	t.Helper()
	changes := make(chan struct{}, 64)
	cfg.FS = fsys
	cfg.Clock = clock
	if cfg.Home == "" {
		cfg.Home = testHome
	}
	if cfg.Debounce == 0 {
		cfg.Debounce = testDebounce
	}
	if cfg.Logger == nil {
		cfg.Logger = newTestLogger(t)
	}
	cfg.OnChange = func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	ix, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix, changes
}

// waitFor returns once cond holds. It re-checks cond after every snapshot the index publishes, so a test does not
// depend on how many publishes a change causes. It fails the test if cond stays false past the timeout, which is a guard
// against a hang rather than a sleep.
func waitFor(t *testing.T, changes <-chan struct{}, cond func() bool) {
	t.Helper()
	timeout := time.After(waitTimeout)
	for !cond() {
		select {
		case <-changes:
		case <-timeout:
			t.Fatal("timed out waiting for the index to reach the expected state")
		}
	}
}

// settle applies the events the file system has delivered so far. It waits until the index has armed its debounce
// window, which only happens once it has received an event, advances the fake clock past the window and then waits for
// cond. Use it after a change that produces at least one event.
func settle(t *testing.T, clock *fake.Clock, changes <-chan struct{}, cond func() bool) {
	t.Helper()
	clock.AwaitTimers(2)
	clock.Advance(testDebounce)
	waitFor(t, changes, cond)
}

// hasPaths reports whether every path is in the snapshot.
func hasPaths(snap *Snapshot, paths ...string) bool {
	have := map[string]bool{}
	for i := range snap.entries {
		have[snap.Path(i)] = true
	}
	for _, p := range paths {
		if !have[p] {
			return false
		}
	}
	return true
}

// pathsOf returns the full paths of every file in a snapshot, in snapshot order.
func pathsOf(snap *Snapshot) []string {
	out := make([]string, 0, snap.Len())
	for i := range snap.entries {
		out = append(out, snap.Path(i))
	}
	return out
}

// namesOf returns the names of the results, in order.
func namesOf(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Name)
	}
	return out
}
