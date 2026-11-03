package files

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// docs and pics are the roots most index tests use.
var (
	docs = filepath.Join(testHome, "Documents")
	pics = filepath.Join(testHome, "Pictures")
)

// startIndex starts ix and waits for its first published snapshot.
func startIndex(t *testing.T, ix *Index, changes <-chan struct{}) {
	t.Helper()
	if err := ix.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, changes, func() bool { return ix.Status().Ready })
}

// hasFiles reports whether the index currently holds every path.
func hasFiles(ix *Index, paths ...string) func() bool {
	return func() bool { return hasPaths(ix.Snapshot(), paths...) }
}

// TestInitialScanIndexesTheRootsAndReportsReady checks the first scan: files under each root, status flags, and a search
// through the public API.
func TestInitialScanIndexesTheRootsAndReportsReady(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf", docs+"/sub/b.txt", pics+"/c.png", testHome+"/Elsewhere/skip.txt")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs, pics}})
	startIndex(t, ix, changes)

	st := ix.Status()
	if st.Scanning || st.FromPersistence || st.Entries != 3 || st.SaveError != "" {
		t.Fatalf("status after the first scan = %+v", st)
	}
	want := []string{docs + "/a.pdf", docs + "/sub/b.txt", pics + "/c.png"}
	if got := sortedCopy(pathsOf(ix.Snapshot())); !slices.Equal(got, sortedCopy(want)) {
		t.Fatalf("indexed = %v, want %v", got, want)
	}
	var names []string
	for batch := range ix.Search(context.Background(), Query{Text: "b.txt"}) {
		if batch.Final {
			names = namesOf(batch.Results)
		}
	}
	if !slices.Equal(names, []string{"b.txt"}) {
		t.Fatalf("search through the index = %v", names)
	}
}

// TestCreateRenameAndDeleteFollowFileSystemEvents drives one event at a time through the debounce window.
func TestCreateRenameAndDeleteFollowFileSystemEvents(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}})
	startIndex(t, ix, changes)

	if err := fsys.WriteFile(docs+"/new.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/new.txt"))

	if err := fsys.Rename(docs+"/new.txt", docs+"/renamed.txt"); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/renamed.txt"))
	if hasPaths(ix.Snapshot(), docs+"/new.txt") {
		t.Fatal("the old name is still indexed after a rename")
	}

	if err := fsys.Remove(docs + "/renamed.txt"); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, func() bool { return !hasPaths(ix.Snapshot(), docs+"/renamed.txt") })
	if !hasPaths(ix.Snapshot(), docs+"/a.pdf") {
		t.Fatal("an unrelated file disappeared")
	}
}

// TestMovingAFolderMovesItsSubtree renames a folder that holds a nested folder: one event, the whole subtree moves.
func TestMovingAFolderMovesItsSubtree(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/old/x.txt", docs+"/old/deep/y.txt")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}, MaxDepth: 3})
	startIndex(t, ix, changes)

	if err := fsys.Rename(docs+"/old", docs+"/new"); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/new/x.txt", docs+"/new/deep/y.txt"))
	if hasPaths(ix.Snapshot(), docs+"/old/x.txt") {
		t.Fatal("files under the old folder remain after the move")
	}
}

// TestDeletingFilesAndFoldersRemovesThem removes a nested file, then the folders, one event at a time.
func TestDeletingFilesAndFoldersRemovesThem(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/box/inner/y.txt", docs+"/box/x.txt")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}, MaxDepth: 3})
	startIndex(t, ix, changes)

	steps := []string{docs + "/box/inner/y.txt", docs + "/box/x.txt"}
	for _, p := range steps {
		if err := fsys.Remove(p); err != nil {
			t.Fatal(err)
		}
		settle(t, clock, changes, func() bool { return !hasPaths(ix.Snapshot(), p) })
	}
	if err := fsys.Remove(docs + "/box/inner"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Remove(docs + "/box"); err != nil {
		t.Fatal(err)
	}
	// The second removal is the one that drops nothing, so wait on the reconcile path to publish the final state.
	ix.Refresh()
	waitFor(t, changes, func() bool { return ix.Snapshot().Len() == 0 })
}

// TestBurstOfChangesConvergesAfterAFullScan creates many files, then asks for a scan; the scan sees all of them.
func TestBurstOfChangesConvergesAfterAFullScan(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/seed.txt")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}})
	startIndex(t, ix, changes)

	var created []string
	for i := range 30 {
		p := fmt.Sprintf("%s/burst-%02d.txt", docs, i)
		if err := fsys.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		created = append(created, p)
	}
	ix.Refresh()
	waitFor(t, changes, hasFiles(ix, created...))
	if got := ix.Snapshot().Len(); got != 31 {
		t.Fatalf("after the burst the index holds %d files, want 31", got)
	}
}

// TestRescansFollowCreateRenameAndDelete changes the file system without events and asks for a scan each time, which is
// how the index repairs what the OS dropped.
func TestRescansFollowCreateRenameAndDelete(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}, MaxDepth: 3})
	startIndex(t, ix, changes)

	steps := []struct {
		name   string
		change func() error
		want   []string
	}{
		{"create", func() error { return fsys.WriteFile(docs+"/b.pdf", []byte("x"), 0o644) }, []string{docs + "/a.pdf", docs + "/b.pdf"}},
		{"rename", func() error { return fsys.Rename(docs+"/b.pdf", docs+"/renamed.pdf") }, []string{docs + "/a.pdf", docs + "/renamed.pdf"}},
		{"move into a folder", func() error {
			if err := fsys.MkdirAll(docs+"/archive", 0o755); err != nil {
				return err
			}
			return fsys.Rename(docs+"/renamed.pdf", docs+"/archive/renamed.pdf")
		}, []string{docs + "/a.pdf", docs + "/archive/renamed.pdf"}},
		{"delete", func() error { return fsys.Remove(docs + "/a.pdf") }, []string{docs + "/archive/renamed.pdf"}},
	}
	for _, step := range steps {
		if err := step.change(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		ix.Refresh()
		want := step.want
		waitFor(t, changes, func() bool {
			return ix.Snapshot().Len() == len(want) && hasPaths(ix.Snapshot(), want...)
		})
	}
}

// TestMissingRootsDropTheirFilesAndAreReported removes a whole root and rescans.
func TestMissingRootsDropTheirFilesAndAreReported(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf", pics+"/b.png")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs, pics}})
	startIndex(t, ix, changes)

	if err := fsys.Remove(docs + "/a.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Remove(docs); err != nil {
		t.Fatal(err)
	}
	ix.Refresh()
	waitFor(t, changes, func() bool {
		st := ix.Status()
		return !st.Scanning && slices.Equal(st.MissingRoots, []string{docs}) && ix.Snapshot().Len() == 1
	})
	if !hasPaths(ix.Snapshot(), pics+"/b.png") {
		t.Fatal("the present root lost its files")
	}
}

// TestRootCreatedAfterStartupIsIndexed starts with a missing root, then creates it: the event on its parent triggers a
// scan that picks it up.
func TestRootCreatedAfterStartupIsIndexed(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, pics+"/b.png")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs, pics}})
	startIndex(t, ix, changes)
	if got := ix.Status().MissingRoots; !slices.Equal(got, []string{docs}) {
		t.Fatalf("missing roots = %v, want Documents", got)
	}

	// MkdirAll raises no event in the fake, as on a platform that reports only the file written into the new folder.
	if err := fsys.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(docs+"/visible.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/visible.txt"))
	if got := ix.Status().MissingRoots; len(got) != 0 {
		t.Fatalf("missing roots after creation = %v, want none", got)
	}
}

// TestUnreadableFolderKeepsItsCachedFiles makes the root unreadable for one scan: its files stay, and the status says
// so. When it becomes readable again the next scan picks up what changed.
func TestUnreadableFolderKeepsItsCachedFiles(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf", docs+"/b.pdf")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}})
	startIndex(t, ix, changes)

	fsys.FailPath(fake.OpReadDir, docs, fs.ErrPermission)
	ix.Refresh()
	waitFor(t, changes, func() bool {
		st := ix.Status()
		return !st.Scanning && slices.Equal(st.UnreadableRoots, []string{docs})
	})
	if !hasPaths(ix.Snapshot(), docs+"/a.pdf", docs+"/b.pdf") || ix.Snapshot().Len() != 2 {
		t.Fatalf("an unreadable root emptied the index: %v", pathsOf(ix.Snapshot()))
	}

	fsys.FailPath(fake.OpReadDir, docs, nil)
	if err := fsys.WriteFile(docs+"/c.pdf", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ix.Refresh()
	waitFor(t, changes, func() bool {
		return len(ix.Status().UnreadableRoots) == 0 && hasPaths(ix.Snapshot(), docs+"/c.pdf")
	})
}

// TestSecondRefreshAfterFirstLeavesTheNewestState checks two scans requested back to back: the index ends on the newest
// state, which is what the single-writer loop must guarantee.
func TestSecondRefreshAfterFirstLeavesTheNewestState(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/old.pdf")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}})
	startIndex(t, ix, changes)

	if err := fsys.WriteFile(docs+"/new.pdf", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ix.Refresh()
	ix.Refresh()
	waitFor(t, changes, func() bool { return hasPaths(ix.Snapshot(), docs+"/new.pdf") && !ix.Status().Scanning })
}

// TestLostWatchIsRegisteredAgainAfterTheRescan breaks the watch under the root: the index notices, scans, watches again,
// and events from then on are applied.
func TestLostWatchIsRegisteredAgainAfterTheRescan(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}})
	startIndex(t, ix, changes)
	before := ix.Status().Generation

	fsys.BreakWatches(docs)
	waitFor(t, changes, func() bool {
		st := ix.Status()
		return st.Generation > before && !st.Scanning
	})

	if err := fsys.WriteFile(docs+"/after.pdf", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/after.pdf"))
}

// TestEventsForIgnoredHiddenAndLinkedPathsAreNotIndexed sends events for paths that must stay out of the index, then a
// sentinel file whose publish proves those events were processed first.
func TestEventsForIgnoredHiddenAndLinkedPathsAreNotIndexed(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, fsys, clock, Config{Roots: []string{docs}})
	startIndex(t, ix, changes)

	if err := fsys.MkdirAll(docs+"/node_modules/pkg", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(docs+"/node_modules/pkg/index.js", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(docs+"/.secret", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fsys.Symlink("/etc", docs+"/link"); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(docs+"/sentinel.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/sentinel.txt"))
	for _, p := range pathsOf(ix.Snapshot()) {
		if strings.Contains(p, "node_modules") || strings.Contains(p, ".secret") || strings.Contains(p, "/link") {
			t.Fatalf("%s must not be indexed", p)
		}
	}
	if ix.Snapshot().Len() != 2 {
		t.Fatalf("index holds %d files, want a.pdf and sentinel.txt", ix.Snapshot().Len())
	}
}

// TestPersistedSnapshotIsServedBeforeTheScan saves an index, starts a new one over the same store, and checks that the
// first published snapshot comes from the store, before any scan.
