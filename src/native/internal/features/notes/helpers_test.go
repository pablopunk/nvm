package notes

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// watchdog bounds a wait for a notice. A wait ends as soon as the notice arrives; this never stands in for one.
const watchdog = 10 * time.Second

// fixture is one notes directory on a fake file system with a fake clock. Opening a fixture again over the same
// file system is how a test restarts the app after a crash.
type fixture struct {
	t     *testing.T
	set   *fake.Set
	dir   string
	st    *store.Store
	notes *Store
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	set := fake.NewSet()
	f := &fixture{t: t, set: set, dir: "/notes"}
	f.restart(false)
	t.Cleanup(func() { _ = f.st.Close() })
	return f
}

// restart opens the store again over the same files, as a new process would. The previous store is abandoned
// without a flush, which is what a crash leaves behind.
func (f *fixture) restart(readOnly bool) {
	f.t.Helper()
	st, err := store.Open(f.set.FS, f.set.Clock, f.dir, store.WithDebounce(WriteDelay))
	if err != nil {
		f.t.Fatalf("open store: %v", err)
	}
	f.st = st
	f.notes = New(st, f.set.Clock, readOnly)
}

// newNote creates a note with the text, saves it and flushes, so later steps start from a saved note on disk.
func (f *fixture) newNote(markdown string) Note {
	f.t.Helper()
	note, err := f.notes.Create(f.set.Clock.Now())
	if err != nil {
		f.t.Fatalf("create note: %v", err)
	}
	if markdown != "" {
		if note, _, err = f.notes.SaveIfUnchanged(note.ID, markdown, note.UpdatedAt, f.set.Clock.Now()); err != nil {
			f.t.Fatalf("save: %v", err)
		}
	}
	if err := f.st.Flush(); err != nil {
		f.t.Fatalf("flush: %v", err)
	}
	return note
}

func mustEqual[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}
