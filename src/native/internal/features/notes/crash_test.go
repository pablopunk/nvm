package notes

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The crash journeys of J10: the app dies at a point of an edit and the next launch must recover the text. A
// crash is a restart over the same files, with the old store abandoned unflushed.

// A crash after the draft is written and before the note is saved: the next open recovers the typed text, and
// no conflict is offered because the draft is based on the saved version still on disk.
func TestCrashAfterTheDraftWriteRecoversTheTextWithoutAConflict(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("saved text")
	sess := f.mustSession(note.ID)
	if err := sess.Edit("saved text, typed before the crash"); err != nil {
		t.Fatal(err)
	}
	f.advanceAwaitingWrite(DraftDelay, draftsFile)

	f.restart(false)
	reopened := f.mustSession(note.ID)
	mustEqual(t, "recovered text", reopened.Text(), "saved text, typed before the crash")
	mustEqual(t, "recovered flag", reopened.Recovered(), true)
	if _, ok := reopened.Conflict(); ok {
		t.Fatal("a draft on the saved version raised a conflict")
	}
}

// A crash while the note save fails: the disk refuses the save, the draft stays, the editor says it is retrying,
// and the next open recovers the text from the draft.
func TestCrashWhileTheNoteSaveFailsRecoversTheDraft(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("saved text")
	autosaved := make(chan struct{}, 4)
	sess, err := f.notes.OpenSession(note.ID, func() { autosaved <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Edit("saved text, then the disk failed"); err != nil {
		t.Fatal(err)
	}
	f.advanceAwaitingWrite(DraftDelay, draftsFile)

	f.set.FS.FailNext(fake.OpWriteFile, errors.New("disk full"))
	f.set.Clock.Advance(AutosaveDelay - DraftDelay)
	f.awaitSignal(autosaved, "autosave attempt")
	mustEqual(t, "status while the save fails", sess.Status(), StatusRetrying)

	f.restart(false)
	reopened := f.mustSession(note.ID)
	mustEqual(t, "recovered text", reopened.Text(), "saved text, then the disk failed")
	if _, ok := reopened.Conflict(); ok {
		t.Fatal("a failed save raised a conflict")
	}
}

// A crash after another instance saved a different version of the note: the draft is now based on an old version
// and its text differs, so the next open offers the conflict with a diff summary of the two texts.
func TestCrashAfterTheNoteChangedElsewhereShowsAConflict(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("line one\nline two")
	sess := f.mustSession(note.ID)
	if err := sess.Edit("line one\nline two, edited"); err != nil {
		t.Fatal(err)
	}
	f.advanceAwaitingWrite(DraftDelay, draftsFile)
	f.changeSavedElsewhere(note.ID, "line one\nline two\nline three")
	f.restart(false)

	reopened := f.mustSession(note.ID)
	conflict, ok := reopened.Conflict()
	if !ok {
		t.Fatal("no conflict after the note changed elsewhere")
	}
	summary := Summarize(conflict.Diff())
	mustEqual(t, "added lines", summary.Added, 1)
	mustEqual(t, "removed lines", summary.Removed, 2)
	if got := reopened.Status(); got != StatusConflict {
		t.Fatalf("status = %q, want the conflict status", got)
	}
}

// A burst of keystrokes inside the debounce window is one write of the draft document.
func TestKeystrokeBurstIsOneDraftWrite(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("")
	sess := f.mustSession(note.ID)
	writes := f.countTempWrites(draftsFile)

	text := ""
	for range 20 {
		text += "x"
		if err := sess.Edit(text); err != nil {
			t.Fatal(err)
		}
		f.set.Clock.Advance(10 * time.Millisecond)
	}
	f.advanceAwaitingWrite(DraftDelay, draftsFile)
	mustEqual(t, "draft writes for the burst", *writes, 1)
	mustEqual(t, "draft text", f.notes.Drafts()[0].Content, text)
}

// The draft is removed only after the note is on disk: the last write of the drafts document comes after the note
// write of the autosave. The draft's own debounced write may come earlier, which is harmless.
func TestSavedNoteIsWrittenBeforeItsDraftIsRemoved(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("")
	sess := f.mustSession(note.ID)
	autosaved := make(chan struct{}, 4)
	sess.onSaved = func() { autosaved <- struct{}{} }
	if err := sess.Edit("ordered"); err != nil {
		t.Fatal(err)
	}
	var (
		mu    sync.Mutex
		order []string
	)
	f.set.FS.AfterWrite(func(path string) error {
		if strings.HasSuffix(path, ".tmp") {
			mu.Lock()
			order = append(order, filepath.Base(path))
			mu.Unlock()
		}
		return nil
	})
	f.set.Clock.Advance(AutosaveDelay)
	f.awaitSignal(autosaved, "autosave")
	mu.Lock()
	defer mu.Unlock()
	noteAt, draftAt := -1, -1
	for i, name := range order {
		switch name {
		case "." + notesFile + ".tmp":
			if noteAt < 0 {
				noteAt = i
			}
		case "." + draftsFile + ".tmp":
			draftAt = i
		}
	}
	if noteAt < 0 || draftAt < noteAt {
		t.Fatalf("write order = %v, want the note written before the final draft write", order)
	}
}
