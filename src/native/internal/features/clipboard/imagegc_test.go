package clipboard

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Cases ported from src/app/electron/clipboard/clipboard-image-sweeper.test.ts, one test per 1.x case in the
// same order, plus the link and folder rules the port adds.

// hashedName returns the file name of the n-th test image: forty hex digits and .png, as the store creates.
func hashedName(n int) string {
	return fmt.Sprintf("%040d.png", n)
}

// hookFS wraps the fake FS with hooks on the calls the sweep makes, so a test can change the world between
// two steps of a sweep.
type hookFS struct {
	platform.FS
	onLstat   func(name string)
	onRemove  func(name string) error
	onReadDir func(name string) ([]fs.DirEntry, error)
}

// Lstat runs the hook, then delegates.
func (f hookFS) Lstat(name string) (fs.FileInfo, error) {
	if f.onLstat != nil {
		f.onLstat(name)
	}
	return f.FS.Lstat(name)
}

// Remove runs the hook, which may fail, and otherwise delegates.
func (f hookFS) Remove(name string) error {
	if f.onRemove != nil {
		return f.onRemove(name)
	}
	return f.FS.Remove(name)
}

// ReadDir runs the hook, which may replace the listing, and otherwise delegates.
func (f hookFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.onReadDir != nil {
		return f.onReadDir(name)
	}
	return f.FS.ReadDir(name)
}

// storeOver returns an image store over the given FS, sharing the harness clock and log.
func (h *harness) storeOver(files platform.FS) *ImageStore {
	return NewImageStore(ImageStoreDeps{Files: files, Clock: h.clock, Dir: imagesDir, Log: h.log})
}

// seedImage writes an image file of the given name with a modification time old enough to be swept.
func (h *harness) seedImage(name string, mtime time.Time) {
	h.files.Seed(filepath.Join(imagesDir, name), []byte("png"))
	h.files.SetModTime(filepath.Join(imagesDir, name), mtime)
}

// oldTime is a modification time past the grace window.
func (h *harness) oldTime() time.Time {
	return h.clock.Now().Add(-GCGrace - time.Minute)
}

// TestGCDeletesOldUnreferencedKeepsReferenced ports 'deletes an old unreferenced image and keeps referenced ones'.
func TestGCDeletesOldUnreferencedKeepsReferenced(t *testing.T) {
	h := newHarness(t)
	referenced, orphan := hashedName(1), hashedName(2)
	h.seedImage(referenced, h.oldTime())
	h.seedImage(orphan, h.oldTime())
	refs := newRefs(referenced)
	result := h.images.GC(refs)
	if result.Deleted != 1 {
		t.Fatalf("result = %+v, want one deletion", result)
	}
	if _, err := h.files.Stat(filepath.Join(imagesDir, orphan)); err == nil {
		t.Fatal("orphan image still present")
	}
	if _, err := h.files.Stat(filepath.Join(imagesDir, referenced)); err != nil {
		t.Fatalf("referenced image removed: %v", err)
	}
}

// TestGCKeysReferencesByFileNotID ports 'keeps a file shared by an image-file entry via imagePath, not id'. An
// entry whose ID is a different hash still references its file by the file name.
func TestGCKeysReferencesByFileNotID(t *testing.T) {
	h := newHarness(t)
	orphan := hashedName(2)
	h.seedImage(orphan, h.oldTime())
	h.mustRemember(Entry{ID: "image:" + strings.Repeat("f", 40), Kind: KindImage, ImageRef: orphan, Bytes: 3})
	result := h.images.GC(h.history)
	if result.Deleted != 0 {
		t.Fatalf("file referenced by an entry was deleted: %+v", result)
	}
}

// TestGCKeepsFilesReferencedOnlyByPersistedHistory ports 'keeps files referenced only by persisted history'.
func TestGCKeepsFilesReferencedOnlyByPersistedHistory(t *testing.T) {
	h := newHarness(t)
	orphan := hashedName(2)
	h.seedImage(orphan, h.oldTime())
	refs := newRefs(hashedName(1))
	refs.persisted[orphan] = true
	if result := h.images.GC(refs); result.Deleted != 0 {
		t.Fatalf("persisted reference ignored: %+v", result)
	}
}

// TestGCAbortsWhenPersistedHistoryUnreadable ports 'aborts without deleting when persisted history is unreadable'.
func TestGCAbortsWhenPersistedHistoryUnreadable(t *testing.T) {
	h := newHarness(t)
	orphan := hashedName(2)
	h.seedImage(orphan, h.oldTime())
	refs := newRefs()
	refs.persistOK = false
	result := h.images.GC(refs)
	if result.Reason != "persisted-unreadable" || result.Deleted != 0 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := h.files.Stat(filepath.Join(imagesDir, orphan)); err != nil {
		t.Fatal("image deleted while the saved history was unreadable")
	}
}

// TestGCNeverDeletesWhenStateUntrusted ports 'never deletes when state was not loaded from a valid file'. The
// real history is opened on a newer snapshot, which makes it untrusted.
func TestGCNeverDeletesWhenStateUntrusted(t *testing.T) {
	h := newHarness(t)
	orphan := hashedName(2)
	h.seedImage(orphan, h.oldTime())
	persist := NewMemoryPersistence()
	persist.SetStored(Snapshot{Version: SnapshotVersion + 1})
	untrusted, err := Open(persist, h.clock, Retention{})
	if err == nil {
		t.Fatal("Open accepted a newer snapshot")
	}
	result := h.images.GC(untrusted)
	if result.Reason != "untrusted-state" || result.Deleted != 0 {
		t.Fatalf("result = %+v", result)
	}
}

// TestGCKeepsFilesNewerThanGrace ports 'keeps files newer than the grace window'.
func TestGCKeepsFilesNewerThanGrace(t *testing.T) {
	h := newHarness(t)
	h.seedImage(hashedName(2), h.clock.Now().Add(-time.Second))
	if result := h.images.GC(newRefs()); result.Deleted != 0 {
		t.Fatalf("recent image deleted: %+v", result)
	}
}

// TestGCIgnoresNonImageNamesFoldersAndLinks ports 'ignores non-image names, directories and symlinks'.
func TestGCIgnoresNonImageNamesFoldersAndLinks(t *testing.T) {
	h := newHarness(t)
	old := h.oldTime()
	h.seedImage("notes.txt", old)
	h.seedImage(strings.Repeat("a", 40)+".jpg", old)
	if err := h.files.MkdirAll(filepath.Join(imagesDir, hashedName(3)), 0o700); err != nil {
		t.Fatal(err)
	}
	h.files.SetModTime(filepath.Join(imagesDir, hashedName(3)), old)
	h.files.Seed("/outside/victim.png", []byte("keep me"))
	h.files.SetModTime("/outside/victim.png", old)
	h.files.SeedSymlink(filepath.Join(imagesDir, hashedName(4)), "/outside/victim.png")
	if result := h.images.GC(newRefs()); result.Deleted != 0 {
		t.Fatalf("GC deleted a non-candidate: %+v", result)
	}
	if _, err := h.files.Stat("/outside/victim.png"); err != nil {
		t.Fatalf("link target removed: %v", err)
	}
}

// TestGCKeepsFileThatGainsReferenceBeforeUnlink ports 'keeps a file that gains a reference between listing and
// unlink'.
func TestGCKeepsFileThatGainsReferenceBeforeUnlink(t *testing.T) {
	h := newHarness(t)
	orphan := hashedName(2)
	h.seedImage(orphan, h.oldTime())
	refs := newRefs()
	lookups := 0
	refs.onLookup = func(name string) {
		if name != orphan {
			return
		}
		lookups++
		if lookups == 2 {
			refs.names[orphan] = true
		}
	}
	result := h.images.GC(refs)
	if result.Deleted != 0 {
		t.Fatalf("file that gained a reference was deleted: %+v", result)
	}
}

// TestGCKeepsFileWhoseMtimeWasBumpedBeforeUnlink ports 'keeps a file whose mtime was bumped by a re-copy before
// unlink'.
func TestGCKeepsFileWhoseMtimeWasBumpedBeforeUnlink(t *testing.T) {
	h := newHarness(t)
	orphan := hashedName(2)
	path := filepath.Join(imagesDir, orphan)
	h.seedImage(orphan, h.oldTime())
	files := hookFS{FS: h.files, onLstat: func(name string) {
		if name == path {
			h.files.SetModTime(path, h.clock.Now())
		}
	}}
	if result := h.storeOver(files).GC(newRefs()); result.Deleted != 0 {
		t.Fatalf("re-copied image deleted: %+v", result)
	}
}

// TestGCSwallowsUnlinkErrorsAndKeepsSweeping ports 'swallows unlink errors and keeps sweeping'.
func TestGCSwallowsUnlinkErrorsAndKeepsSweeping(t *testing.T) {
	h := newHarness(t)
	first, second := hashedName(2), hashedName(4)
	h.seedImage(first, h.oldTime())
	h.seedImage(second, h.oldTime())
	h.files.FailNext(fake.OpRemove, errors.New("EPERM"))
	result := h.images.GC(newRefs())
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1 after the first unlink failed", result.Deleted)
	}
	if got := logMessages(h); !containsString(got, "clipboard.images.sweep.unlinkFailed") {
		t.Fatalf("log = %v, want the unlink failure", got)
	}
}

// TestGCCapsDeletionsPerRun ports 'caps deletions per run'.
func TestGCCapsDeletionsPerRun(t *testing.T) {
	h := newHarness(t)
	for n := range GCMaxDeletes + 20 {
		h.seedImage(hashedName(n+10), h.oldTime())
	}
	if result := h.images.GC(newRefs()); result.Deleted != GCMaxDeletes {
		t.Fatalf("deleted = %d, want the cap %d", result.Deleted, GCMaxDeletes)
	}
}

// TestGCStopsAtTheDeadline ports 'stops at the deadline': each delete takes a minute of the clock, so the second
// candidate is past the fifteen-second budget.
func TestGCStopsAtTheDeadline(t *testing.T) {
	h := newHarness(t)
	h.seedImage(hashedName(2), h.oldTime())
	h.seedImage(hashedName(5), h.oldTime())
	files := hookFS{FS: h.files, onRemove: func(name string) error {
		h.clock.Advance(time.Minute)
		return h.files.Remove(name)
	}}
	if result := h.storeOver(files).GC(newRefs()); result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1 before the deadline", result.Deleted)
	}
}

// TestGCSharesOneInFlightSweep ports 'shares one in-flight sweep and survives failures': a second caller joins
// the sweep in progress, the folder is listed once, and the failure is reported as a skipped sweep.
func TestGCSharesOneInFlightSweep(t *testing.T) {
	h := newHarness(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	reads := 0
	files := hookFS{FS: h.files, onReadDir: func(string) ([]fs.DirEntry, error) {
		reads++
		close(entered)
		<-release
		return nil, errors.New("boom")
	}}
	store := h.storeOver(files)
	refs := newRefs()
	first := make(chan GCResult, 1)
	second := make(chan GCResult, 1)
	go func() { first <- store.GC(refs) }()
	<-entered
	go func() { second <- store.GC(refs) }()
	for store.joinedCount() == 0 {
		runtime.Gosched()
	}
	close(release)
	a, b := <-first, <-second
	if reads != 1 {
		t.Fatalf("folder listed %d times, want once", reads)
	}
	if a != b || a.Reason != "error" {
		t.Fatalf("results %+v and %+v, want the same skipped result", a, b)
	}
	if got := logMessages(h); !containsString(got, "clipboard.images.sweep.failed") {
		t.Fatalf("log = %v, want the sweep failure", got)
	}
}

// TestGCPropertyDeletesOnlyOldUnreferencedFiles runs a seeded folder of image files with random ages and
// references, sweeps once, and checks the rule on every file: a file is removed exactly when it is old and
// unreferenced, and the sweep reports that many deletions.
func TestGCPropertyDeletesOnlyOldUnreferencedFiles(t *testing.T) {
	h := newHarness(t)
	rng := rand.New(rand.NewPCG(3, 4))
	refs := newRefs()
	type stored struct {
		name             string
		old, referenced bool
	}
	var files []stored
	wantDeleted := 0
	for n := range 200 {
		f := stored{name: hashedName(n + 1), old: rng.IntN(2) == 0, referenced: rng.IntN(3) == 0}
		mtime := h.clock.Now()
		if f.old {
			mtime = h.oldTime()
		}
		h.seedImage(f.name, mtime)
		if f.referenced {
			refs.names[f.name] = true
		}
		if f.old && !f.referenced {
			wantDeleted++
		}
		files = append(files, f)
	}
	result := h.images.GC(refs)
	if result.Deleted != wantDeleted {
		t.Fatalf("deleted %d, want %d", result.Deleted, wantDeleted)
	}
	for _, f := range files {
		_, err := h.files.Stat(filepath.Join(imagesDir, f.name))
		if gone := err != nil; gone != (f.old && !f.referenced) {
			t.Fatalf("%s: removed = %v, old = %v, referenced = %v", f.name, gone, f.old, f.referenced)
		}
	}
}

// TestGCMissingFolderIsNothingToSweep: a store whose folder does not exist yet has nothing to collect.
func TestGCMissingFolderIsNothingToSweep(t *testing.T) {
	h := newHarness(t)
	if result := h.images.GC(newRefs()); result.Status != GCSwept || result.Deleted != 0 {
		t.Fatalf("result = %+v", result)
	}
}

// logMessages returns the messages of the entries the harness logger kept.
func logMessages(h *harness) []string {
	entries := h.log.Recent(1000)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message)
	}
	return out
}

// containsString reports whether list holds value.
func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
