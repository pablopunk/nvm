package files

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// TestSnapshotSwapIsAtomicForConcurrentReaders keeps one batch of files in the index at a time and swaps batches while
// readers search. Every snapshot a reader sees must hold exactly one whole batch. Run under -race.
func TestSnapshotSwapIsAtomicForConcurrentReaders(t *testing.T) {
	const batches = 200
	const perBatch = 50
	ix, err := New(Config{Roots: []string{docs}, Home: testHome, FS: fake.NewFS(), Clock: fake.NewClock()})
	if err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				if err := checkOneWholeBatch(ix.Snapshot(), perBatch); err != nil {
					errs <- err
					return
				}
				for batch := range ix.Search(context.Background(), Query{Text: "batch", Limit: MaxLimit}) {
					if batch.Final && len(batch.Results) != 0 && len(batch.Results) != perBatch {
						errs <- fmt.Errorf("search saw %d results, want one whole batch of %d", len(batch.Results), perBatch)
						return
					}
				}
			}
		}()
	}
	for k := 1; k <= batches; k++ {
		d := newDelta()
		d.dropSubtree(docs)
		for i := range perBatch {
			d.add = append(d.add, record{dir: docs, name: fmt.Sprintf("batch%03d-%02d.txt", k, i)})
		}
		old := ix.Snapshot()
		ix.snap.Store(d.apply(old, old.generation+1, ix.configKey))
	}
	stop.Store(true)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// checkOneWholeBatch reports an error unless snap holds nothing, or exactly one batch of perBatch files.
func checkOneWholeBatch(snap *Snapshot, perBatch int) error {
	batchOf := map[string]int{}
	for i := range snap.entries {
		name := snap.Name(i)
		batchOf[name[:strings.IndexByte(name, '-')]]++
	}
	switch {
	case len(batchOf) == 0:
		return nil
	case len(batchOf) == 1:
		for _, n := range batchOf {
			if n == perBatch {
				return nil
			}
			return fmt.Errorf("snapshot holds %d of a batch of %d: a half-built snapshot was published", n, perBatch)
		}
	}
	return fmt.Errorf("snapshot mixes %d batches: a half-built snapshot was published", len(batchOf))
}

// TestCloseLeavesNoIndexGoroutines starts an index, lets it scan and watch, closes it and checks that every goroutine
// it named has returned.
func TestCloseLeavesNoIndexGoroutines(t *testing.T) {
	fsys := fake.NewFS()
	seedFiles(fsys, docs+"/a.pdf")
	store := &MemoryPersistence{}
	ix, changes := newTestIndex(t, fsys, fake.NewClock(), Config{Roots: []string{docs}, Persistence: store})
	startIndex(t, ix, changes)
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}
	waitGoroutinesGone(t, "files.index", "files.scan", "files.scan.worker", "files.watch", "files.save")
}

// TestStartTwiceAndCloseBeforeStartAreHarmless covers the lifecycle edges.
func TestStartTwiceAndCloseBeforeStartAreHarmless(t *testing.T) {
	ix, _ := newTestIndex(t, fake.NewFS(), fake.NewClock(), Config{Roots: []string{docs}})
	if err := ix.Close(); err != nil {
		t.Fatalf("Close before Start: %v", err)
	}
	fresh, _ := newTestIndex(t, fake.NewFS(), fake.NewClock(), Config{Roots: []string{docs}})
	if err := fresh.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := fresh.Start(context.Background()); err == nil {
		t.Fatal("a second Start must fail")
	}
}

// TestNewRejectsConfigurationsThatCannotIndex checks the two configuration errors New reports.
func TestNewRejectsConfigurationsThatCannotIndex(t *testing.T) {
	if _, err := New(Config{Roots: []string{docs}, Home: testHome}); err == nil {
		t.Error("an index without a file system or clock must be refused")
	}
	if _, err := New(Config{Roots: []string{"relative"}, Home: testHome, FS: fake.NewFS(), Clock: fake.NewClock()}); err == nil {
		t.Error("an index without an absolute root must be refused")
	}
}

// TestFilePersistenceWritesAtomicallyThroughTheFSPort round-trips a snapshot through a file and checks no temporary
// file is left behind.
func TestFilePersistenceWritesAtomicallyThroughTheFSPort(t *testing.T) {
	fsys := fake.NewFS()
	if err := fsys.MkdirAll("/state", 0o755); err != nil {
		t.Fatal(err)
	}
	store := FilePersistence{FS: fsys, Path: "/state/files.idx"}
	if _, err := store.Load(); err == nil {
		t.Fatal("loading before any save must fail")
	}
	if err := store.Save([]byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Save([]byte("second")); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || string(got) != "second" {
		t.Fatalf("Load = %q, %v; want the second save", got, err)
	}
	for _, p := range fsys.Paths() {
		if strings.HasSuffix(p, ".tmp") {
			t.Fatalf("a temporary file was left behind: %s", p)
		}
	}
}

// TestPlatformFSSatisfiesTheIndex is a compile-time guard that the fake file system is what the index is built on.
var _ platform.FS = (*countingFS)(nil)

// countingFS counts Lstat calls, to check that an event re-reads only the file it names.
type countingFS struct {
	*fake.FS
	lstats atomic.Int64
}

// Lstat counts the call and delegates.
func (c *countingFS) Lstat(name string) (fs.FileInfo, error) {
	c.lstats.Add(1)
	return c.FS.Lstat(name)
}

// TestAnEventRereadsOnlyTheFileItNames counts reads after a scan: one created file costs one metadata read.
func TestAnEventRereadsOnlyTheFileItNames(t *testing.T) {
	inner := fake.NewFS()
	for i := range 100 {
		inner.Seed(fmt.Sprintf("%s/f%03d.txt", docs, i), []byte("x"))
	}
	counting := &countingFS{FS: inner}
	clock := fake.NewClock()
	ix, changes := newTestIndex(t, inner, clock, Config{Roots: []string{docs}})
	ix.cfg.FS = counting
	startIndex(t, ix, changes)
	counting.lstats.Store(0)

	if err := inner.WriteFile(docs+"/new.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	settle(t, clock, changes, hasFiles(ix, docs+"/new.txt"))
	if got := counting.lstats.Load(); got != 1 {
		t.Fatalf("one created file cost %d metadata reads, want 1", got)
	}
}

// newTestLogger returns a logger that keeps its entries in memory and does not write to stderr. It runs on a clock of
// its own so it never disturbs the clock the index is tested with.
func newTestLogger(t *testing.T) *logx.Logger {
	t.Helper()
	logger, err := logx.New(nil, fake.NewClock(), "", logx.Options{Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger
}

// hasLogMessage reports whether the logger has recorded a message.
func hasLogMessage(logger *logx.Logger, message string) bool {
	for _, entry := range logger.Recent(100) {
		if entry.Message == message {
			return true
		}
	}
	return false
}

// waitGoroutinesGone waits until the named goroutines have all returned. safego decrements its count after a goroutine's
// body returns, which can be a moment after the body signalled its owner, so the check yields the processor rather
// than asserting at once. The loop is bounded, so a leak still fails.
func waitGoroutinesGone(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		for range 100_000 {
			if safego.Watching().Live(name) == 0 {
				break
			}
			runtime.Gosched()
		}
		if live := safego.Watching().Live(name); live != 0 {
			t.Errorf("%d %s goroutines still running after Close", live, name)
		}
	}
}
