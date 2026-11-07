package notes

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/store"
)

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
	owner, err := store.Open(f.set.FS, f.set.Clock, f.dir, store.WithLease(lease), store.WithDebounce(WriteDelay))
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
	reader, err := store.Open(f.set.FS, f.set.Clock, f.dir, store.WithDebounce(WriteDelay))
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
	if _, _, err := readOnly.SaveIfUnchanged(note.ID, "nope", note.UpdatedAt, f.set.Clock.Now()); !errors.Is(err, store.ErrReadOnly) {
		t.Fatalf("save on the read-only instance = %v, want ErrReadOnly", err)
	}
	if _, err := ownerNotes.saveText(note.ID, "owner wrote", f.set.Clock.Now().Add(time.Second)); err != nil {
		t.Fatalf("owner can still write: %v", err)
	}
}
