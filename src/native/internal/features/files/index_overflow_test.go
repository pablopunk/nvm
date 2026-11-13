package files

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestOverflowMarksTheFolderDegradedAndReconciles covers the overflow path: an overflow event asks for a full
// reconcile and keeps the folder out of later watches, so the periodic scan is the only thing that refreshes it.
func TestOverflowMarksTheFolderDegradedAndReconciles(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, filepath.Join(testHome, "Documents/a.txt"))
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{filepath.Join(testHome, "Documents")}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := &indexLoop{ix: ix, ctx: ctx, watches: map[string]context.CancelFunc{}}
	defer func() {
		l.stopWatches()
		ix.watchers.Wait()
	}()

	l.syncWatches()
	if got := watchedTargets(l); len(got) != 1 {
		t.Fatalf("before the overflow, watches = %v, want the Documents folder", got)
	}

	l.noteEvent(platform.Event{Op: platform.EventOverflow, Path: filepath.Join(testHome, "Documents")})
	if !l.pending.full {
		t.Fatal("an overflow must ask for a full reconcile")
	}
	if !l.degraded[filepath.Join(testHome, "Documents")] {
		t.Fatal("an overflow must mark its folder degraded")
	}

	l.watchLost(filepath.Join(testHome, "Documents"))
	l.syncWatches()
	if got := watchedTargets(l); len(got) != 0 {
		t.Fatalf("a degraded folder must not be watched again, got watches %v", got)
	}
}

// TestWatchCountIsCapped checks that an index with more roots than maxWatchTargets starts only that many watches and
// leaves the rest to the reconcile.
func TestWatchCountIsCapped(t *testing.T) {
	fsys := fake.NewFS()
	var roots []string
	for i := 0; i < maxWatchTargets+8; i++ {
		root := fmt.Sprintf("%s/root%02d", testHome, i)
		seedFiles(fsys, filepath.Join(root, "x.txt"))
		roots = append(roots, root)
	}
	ix, _ := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: roots})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l := &indexLoop{ix: ix, ctx: ctx, watches: map[string]context.CancelFunc{}}
	defer func() {
		l.stopWatches()
		ix.watchers.Wait()
	}()

	l.syncWatches()
	if got := len(l.watches); got != maxWatchTargets {
		t.Fatalf("watches = %d, want the cap of %d", got, maxWatchTargets)
	}
}

// TestCappedPersistenceRefusesOversizedSnapshots checks both directions of the size cap and that a normal save round
// trips through the port.
func TestCappedPersistenceRefusesOversizedSnapshots(t *testing.T) {
	fsys := fake.NewFS()
	if err := fsys.MkdirAll(testHome, 0o755); err != nil {
		t.Fatal(err)
	}
	p := CappedFilePersistence{FS: fsys, Path: filepath.Join(testHome, "files-index.bin")}
	if _, err := p.Load(); err == nil {
		t.Fatal("loading a missing snapshot must fail")
	}
	if err := p.Save([]byte("snapshot")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := p.Load()
	if err != nil || string(got) != "snapshot" {
		t.Fatalf("Load = %q, %v; want the saved bytes", got, err)
	}
	if err := p.Save(make([]byte, MaxSnapshotBytes+1)); err == nil {
		t.Fatal("saving past the cap must fail")
	}
}
