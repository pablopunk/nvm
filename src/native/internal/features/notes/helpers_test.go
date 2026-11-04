package notes

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// watchdog bounds a wait for a write. Waits end as soon as the write happens; this never stands in for one.
const watchdog = 10 * time.Second

// fixture is one notes directory on a fake file system with a fake clock. Opening a fixture again over the same
// file system is how a test restarts the app after a crash.
type fixture struct {
	t     *testing.T
	set   *fake.Set
	dir   string
	saved chan string
	st    *store.Store
	notes *Store
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	set := fake.NewSet()
	f := &fixture{t: t, set: set, dir: "/notes", saved: make(chan string, 256)}
	f.restart(false)
	t.Cleanup(func() { _ = f.st.Close() })
	return f
}

// restart opens the store again over the same files, as a new process would. The previous store is abandoned
// without a flush, which is what a crash leaves behind.
func (f *fixture) restart(readOnly bool) {
	f.t.Helper()
	st, err := store.Open(f.set.FS, f.set.Clock, f.dir,
		store.WithDebounce(DraftDelay),
		store.WithOnSaved(func(path string) {
			select {
			case f.saved <- path:
			default:
			}
		}))
	if err != nil {
		f.t.Fatalf("open store: %v", err)
	}
	f.st = st
	f.notes = New(st, f.set.Clock, readOnly)
}

// advanceAwaitingWrite moves the clock by d and waits for the write of name that the move causes. Write notices
// from earlier steps are discarded first, so the wait cannot end on a write that happened before the step.
func (f *fixture) advanceAwaitingWrite(d time.Duration, name string) {
	f.t.Helper()
	for drained := false; !drained; {
		select {
		case <-f.saved:
		default:
			drained = true
		}
	}
	f.set.Clock.Advance(d)
	f.awaitWrite(name)
}

// awaitWrite blocks until the store has written the file with the given base name.
func (f *fixture) awaitWrite(name string) {
	f.t.Helper()
	want := filepath.Join(f.dir, name)
	deadline := time.After(watchdog)
	for {
		select {
		case path := <-f.saved:
			if path == want {
				return
			}
		case <-deadline:
			f.t.Fatalf("%s was never written", name)
		}
	}
}

// awaitSignal blocks until the signal arrives, failing the test after the watchdog.
func (f *fixture) awaitSignal(signal <-chan struct{}, what string) {
	f.t.Helper()
	select {
	case <-signal:
	case <-time.After(watchdog):
		f.t.Fatalf("%s never happened", what)
	}
}

// countTempWrites counts the writes of a document's temporary file, which every save goes through.
func (f *fixture) countTempWrites(name string) *int {
	count := new(int)
	want := filepath.Join(f.dir, "."+name+".tmp")
	f.set.FS.AfterWrite(func(path string) error {
		if path == want {
			*count++
		}
		return nil
	})
	return count
}

// newNote creates a note with the text, saves it through a session and flushes, so later tests start from a
// saved note on disk.
func (f *fixture) newNote(markdown string) Note {
	f.t.Helper()
	note, err := f.notes.Create(f.set.Clock.Now())
	if err != nil {
		f.t.Fatalf("create note: %v", err)
	}
	if markdown != "" {
		autosaved := make(chan struct{}, 8)
		sess, err := f.notes.OpenSession(note.ID, func() { autosaved <- struct{}{} })
		if err != nil {
			f.t.Fatalf("open session: %v", err)
		}
		if err := sess.Edit(markdown); err != nil {
			f.t.Fatalf("edit: %v", err)
		}
		f.set.Clock.Advance(AutosaveDelay)
		f.awaitSignal(autosaved, "autosave")
		if err := sess.Close(); err != nil {
			f.t.Fatalf("close: %v", err)
		}
		note, _ = f.notes.Get(note.ID)
	}
	if err := f.st.Flush(); err != nil {
		f.t.Fatalf("flush: %v", err)
	}
	return note
}

// mustSession opens a session or fails the test.
func (f *fixture) mustSession(id string) *Session {
	f.t.Helper()
	sess, err := f.notes.OpenSession(id, nil)
	if err != nil {
		f.t.Fatalf("open session %s: %v", id, err)
	}
	f.t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func mustEqual[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}
