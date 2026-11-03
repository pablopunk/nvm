package files

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPersistedSnapshotIsServedBeforeTheScan(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf", docs+"/b.pdf")
	store := &MemoryPersistence{}
	first, firstChanges := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}, Persistence: store})
	startIndex(t, first, firstChanges)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if store.Saves() == 0 {
		t.Fatal("closing the index did not save its snapshot")
	}

	observed := make(chan observation, 8)
	var second *Index
	cfg := Config{Roots: []string{docs}, Persistence: store, FS: fsys, Clock: fake.NewClock(), Home: testHome, Debounce: testDebounce}
	cfg.OnChange = func() { observed <- observe(second) }
	second, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := second.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstPublish := <-observed
	if !firstPublish.fromPersistence || firstPublish.entries != 2 {
		t.Fatalf("first publish = %+v, want the two stored files from the store", firstPublish)
	}
	if got := <-observed; got.fromPersistence {
		t.Fatalf("second publish = %+v, want the scan result", got)
	}
}

// observation is what a test records about one published snapshot.
type observation struct {
	fromPersistence bool
	entries         int
}

// observe reads the state of ix after a publish.
func observe(ix *Index) observation {
	st := ix.Status()
	return observation{fromPersistence: st.FromPersistence, entries: ix.Snapshot().Len()}
}

// TestCorruptPersistedSnapshotFallsBackToAFreshScan plants garbage in the store: the index logs it, ignores it and
// scans, and nothing panics.
func TestCorruptPersistedSnapshotFallsBackToAFreshScan(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	store := &MemoryPersistence{}
	store.SetRaw([]byte("NVMFIDX1 this is not a snapshot"))
	logs := newTestLogger(t)
	observed := make(chan observation, 8)
	var ix *Index
	cfg := Config{Roots: []string{docs}, Persistence: store, FS: fsys, Clock: fake.NewClock(), Home: testHome, Logger: logs}
	cfg.OnChange = func() { observed <- observe(ix) }
	ix, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	if err := ix.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-observed; got.fromPersistence || got.entries != 1 {
		t.Fatalf("first publish = %+v, want a fresh scan of one file", got)
	}
	if !hasLogMessage(logs, "files.index.snapshot-discarded") {
		t.Fatal("the discarded snapshot was not logged")
	}
}

// TestSnapshotBuiltForAnotherConfigurationIsNotServed changes the depth between runs: the stored snapshot is ignored.
func TestSnapshotBuiltForAnotherConfigurationIsNotServed(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	store := &MemoryPersistence{}
	first, firstChanges := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}, MaxDepth: 2, Persistence: store})
	startIndex(t, first, firstChanges)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	logs := newTestLogger(t)
	observed := make(chan observation, 8)
	var second *Index
	cfg := Config{Roots: []string{docs}, MaxDepth: 4, Persistence: store, FS: fsys, Clock: fake.NewClock(), Home: testHome, Logger: logs}
	cfg.OnChange = func() { observed <- observe(second) }
	second, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	if err := second.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-observed; got.fromPersistence {
		t.Fatalf("a snapshot for another depth was served: %+v", got)
	}
	if !hasLogMessage(logs, "files.index.snapshot-config-changed") {
		t.Fatal("the configuration change was not logged")
	}
}
