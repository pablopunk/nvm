package files

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// newWatchLoop returns a loop over ix with its watch maps ready, and stops its watches when the test ends.
func newWatchLoop(t *testing.T, ix *Index) *indexLoop {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	l := &indexLoop{ix: ix, ctx: ctx, watches: map[string]context.CancelFunc{}}
	t.Cleanup(func() {
		cancel()
		l.stopWatches()
		ix.watchers.Wait()
	})
	return l
}

// countLogMessages returns how many recorded entries carry message.
func countLogMessages(logger *logx.Logger, message string) int {
	n := 0
	for _, entry := range logger.Recent(1000) {
		if entry.Message == message {
			n++
		}
	}
	return n
}

// TestWatchFollowsTheScanAndSkipsWhatTheScanSkips seeds a root with folders the scan skips (node_modules, .git, Library,
// hidden folders) and folders below the depth limit. Only the four entries the scan reads may be counted against the
// entry limit; the other fifteen would exceed it.
func TestWatchFollowsTheScanAndSkipsWhatTheScanSkips(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys,
		filepath.Join(docs, "a.txt"),
		filepath.Join(docs, "sub/deep/x.txt"),
		filepath.Join(docs, "sub/deep/er/file.txt"),
		filepath.Join(docs, "node_modules/pkg/index.js"),
		filepath.Join(docs, ".git/HEAD"),
		filepath.Join(docs, "Library/cache.db"),
		filepath.Join(docs, ".cache/y.txt"),
	)
	fsys.SetWatchEntryLimit(4)
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}, MaxDepth: 2})
	l := newWatchLoop(t, ix)

	l.syncWatches()
	if got := watchedTargets(l); len(got) != 1 || got[0] != docs {
		t.Fatalf("watches = %v, want only Documents", got)
	}
	if l.levels[docs] != 2 {
		t.Fatalf("Documents settled at level %d, want the scan depth 2", l.levels[docs])
	}
	if countLogMessages(ix.log, "files.watch.narrowed") != 0 {
		t.Fatal("a tree that fits the limit at the scan depth must not be narrowed")
	}
}

// TestWatchNarrowsToTheLevelsThatFitAndLogsOnce checks the fallback when the scan's depth does not fit the entry limit:
// the root is watched one level shallower, the step is logged at info once, and a re-watch after the OS dropped the
// watch starts at the level it settled on, without logging again.
func TestWatchNarrowsToTheLevelsThatFitAndLogsOnce(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys,
		filepath.Join(docs, "a.txt"),
		filepath.Join(docs, "x/y.txt"),
		filepath.Join(docs, "x/z/w.txt"),
	)
	fsys.SetWatchEntryLimit(4)
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}, MaxDepth: 2})
	l := newWatchLoop(t, ix)

	l.syncWatches()
	if got := watchedTargets(l); len(got) != 1 {
		t.Fatalf("watches = %v, want Documents watched narrowed", got)
	}
	if l.levels[docs] != 1 {
		t.Fatalf("Documents settled at level %d, want 1", l.levels[docs])
	}

	l.watchLost(docs)
	l.syncWatches()
	if countLogMessages(ix.log, "files.watch.narrowed") != 1 {
		t.Fatalf("narrowed logged %d times, want once", countLogMessages(ix.log, "files.watch.narrowed"))
	}
	if countLogMessages(ix.log, "files.watch.failed") != 0 {
		t.Fatal("hitting the entry limit must not log a watch failure")
	}
}

// TestWatchUnwatchableRootIsReconciledAndLoggedOnce checks a root that fits no level under the entry limit: it is not
// watched, the reconcile scan covers it, and repeated syncs neither retry the watch nor log again.
func TestWatchUnwatchableRootIsReconciledAndLoggedOnce(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, filepath.Join(docs, "a.txt"), filepath.Join(docs, "b.txt"), filepath.Join(docs, "c.txt"))
	fsys.SetWatchEntryLimit(2)
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}, MaxDepth: 2})
	l := newWatchLoop(t, ix)

	for range 3 {
		l.syncWatches()
	}
	if got := watchedTargets(l); len(got) != 0 {
		t.Fatalf("watches = %v, want none for a root past the limit at every level", got)
	}
	if !l.unwatchable[docs] {
		t.Fatal("a root that fits no level must be marked unwatchable")
	}
	if countLogMessages(ix.log, "files.watch.unwatched") != 1 {
		t.Fatalf("unwatched logged %d times, want once", countLogMessages(ix.log, "files.watch.unwatched"))
	}
	if countLogMessages(ix.log, "files.watch.failed") != 0 {
		t.Fatal("the entry limit must not log a warning on each reconcile")
	}
}

// TestMissingRootWatchKeepsOnlyThePathToTheRoot checks that a missing root is watched through its closest existing
// folder without watching that folder's other contents: a large folder beside the root must not count against the limit.
func TestMissingRootWatchKeepsOnlyThePathToTheRoot(t *testing.T) {
	fsys := fake.NewFS()
	pictures := filepath.Join(testHome, "Pictures")
	for _, name := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
		seedFiles(fsys, filepath.Join(pictures, name+".png"))
	}
	fsys.SetWatchEntryLimit(2)
	missing := filepath.Join(testHome, "Work", "Documents")
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{missing}})
	l := newWatchLoop(t, ix)

	l.syncWatches()
	if got := watchedTargets(l); len(got) != 1 || got[0] != testHome {
		t.Fatalf("watches = %v, want the home folder for a missing root", got)
	}
	if len(l.unwatchable) != 0 {
		t.Fatal("a missing root's ancestor must fit the limit when only its path is watched")
	}
}

// TestUnreadableFoldersAreLoggedOnceAtInfo checks that a folder the scan cannot read is named at info once, not warned
// about on every scan, and that the index keeps the state of the folders it could not read.
func TestUnreadableFoldersAreLoggedOnceAtInfo(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, filepath.Join(docs, "a.txt"))
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}})
	unreadable := []string{filepath.Join(docs, "Locked")}

	for range 3 {
		ix.commitScan(scanOutcome{result: scanResult{unreadable: unreadable}})
	}
	if countLogMessages(ix.log, "files.scan.unreadable") != 1 {
		t.Fatalf("unreadable logged %d times, want once", countLogMessages(ix.log, "files.scan.unreadable"))
	}
	for _, entry := range ix.log.Recent(1000) {
		if entry.Message == "files.scan.unreadable" && entry.Level != logx.Info {
			t.Fatalf("unreadable logged at %v, want info", entry.Level)
		}
	}
	if got := ix.Status().UnreadableRoots; len(got) != 1 {
		t.Fatalf("status unreadable roots = %v, want the one folder", got)
	}
}
