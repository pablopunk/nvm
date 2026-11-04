package notes

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Ported from extension-draft-persistence.test.ts, case "flush persists a debounced editor draft and retries a
// failed write": a failed write is reported by the flush, and the next flush writes the same draft.
func TestDraftFlushRetriesAfterAFailedWrite(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("")
	sess := f.mustSession(note.ID)
	f.set.FS.FailNext(fake.OpWriteFile, errors.New("disk unavailable"))
	if err := sess.Edit("keep this draft"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.Flush(); err == nil || !strings.Contains(err.Error(), "disk unavailable") {
		t.Fatalf("flush with a failing disk = %v, want the disk error", err)
	}
	if err := f.st.Flush(); err != nil {
		t.Fatalf("flush after the disk recovers = %v", err)
	}
	saved, err := f.set.FS.ReadFile(filepath.Join(f.dir, draftsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "keep this draft") {
		t.Fatalf("draft file does not hold the text: %s", saved)
	}
}

// Quarantine: a notes file that is not an envelope is moved aside, the list starts empty, and the store says so
// through its notice channel, which the app shows as a toast.
func TestCorruptNotesFileIsQuarantinedWithANotice(t *testing.T) {
	f := newFixture(t)
	f.set.FS.Seed(filepath.Join(f.dir, notesFile), []byte("not an envelope"))
	f.restart(false)
	mustEqual(t, "notes after quarantine", len(f.notes.List()), 0)
	select {
	case notice := <-f.st.Notices():
		mustEqual(t, "notice kind", notice.Kind, store.Quarantined)
		if !strings.Contains(notice.Path, ".corrupt-") {
			t.Fatalf("notice path %q is not the quarantined copy", notice.Path)
		}
	case <-time.After(watchdog):
		t.Fatal("no quarantine notice")
	}
}

// Two instances: the second one cannot take the lease, opens the same notes read-only, and writes nothing.
func TestSecondInstanceOpensTheNotesReadOnly(t *testing.T) {
	f := newFixture(t)
	lease, err := store.AcquireLease(f.set.FS, f.set.Clock, f.dir, store.WithHeartbeat(0, time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Open(f.set.FS, f.set.Clock, f.dir, store.WithLease(lease), store.WithDebounce(DraftDelay))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	ownerNotes := New(owner, f.set.Clock, false)
	note, err := ownerNotes.Create(f.set.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Flush(); err != nil {
		t.Fatal(err)
	}

	if _, err := store.AcquireLease(f.set.FS, f.set.Clock, f.dir, store.WithHeartbeat(0, time.Minute)); !errors.Is(err, store.ErrLeaseHeld) {
		t.Fatalf("second lease = %v, want ErrLeaseHeld", err)
	}
	reader, err := store.Open(f.set.FS, f.set.Clock, f.dir, store.WithDebounce(DraftDelay))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	readOnly := New(reader, f.set.Clock, true)
	if !readOnly.ReadOnly() {
		t.Fatal("second instance is not read-only")
	}
	mustEqual(t, "notes visible", len(readOnly.List()), 1)
	if _, err := readOnly.Create(f.set.Clock.Now()); !errors.Is(err, store.ErrReadOnly) {
		t.Fatalf("create on the read-only instance = %v, want ErrReadOnly", err)
	}
	sess, err := readOnly.OpenSession(note.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, "status", sess.Status(), StatusReadOnly)
	if err := sess.Edit("nope"); !errors.Is(err, store.ErrReadOnly) {
		t.Fatalf("edit on the read-only instance = %v, want ErrReadOnly", err)
	}
	if _, err := ownerNotes.saveText(note.ID, "owner wrote", f.set.Clock.Now().Add(time.Second)); err != nil {
		t.Fatalf("owner can still write: %v", err)
	}
}

// changeSavedElsewhere saves a new text for a note the way another instance would, and flushes it to disk.
func (f *fixture) changeSavedElsewhere(id, markdown string) {
	f.t.Helper()
	if _, err := f.notes.saveText(id, markdown, f.set.Clock.Now().Add(time.Second)); err != nil {
		f.t.Fatalf("save elsewhere: %v", err)
	}
	if err := f.st.Flush(); err != nil {
		f.t.Fatalf("flush elsewhere: %v", err)
	}
}
