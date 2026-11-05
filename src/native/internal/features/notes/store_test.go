package notes

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// The list is the notes most recently saved first, so the first row is the note the toggle command opens.
func TestListIsMostRecentlySavedFirstAndToggleOpensIt(t *testing.T) {
	f := newFixture(t)
	f.newNote("older")
	f.set.Clock.Advance(1)
	newer := f.newNote("newer")
	mustEqual(t, "first in the list", f.notes.List()[0].ID, newer.ID)
	mustEqual(t, "most recent note", mustRecent(t, f).ID, newer.ID)
}

// The note an empty store opens is created, and it is the one a second call returns.
func TestMostRecentOrCreateCreatesOnceWhenEmpty(t *testing.T) {
	f := newFixture(t)
	first := mustRecent(t, f)
	mustEqual(t, "notes after the first call", len(f.notes.List()), 1)
	mustEqual(t, "second call returns the same note", mustRecent(t, f).ID, first.ID)
}

// A save on the saved version writes the text and moves the version on. A save made on an older version is a
// conflict: it writes nothing and returns the newer note.
func TestSaveIfUnchangedWritesOnTheSavedVersionAndConflictsOnANewerOne(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("first")
	saved, conflict, err := f.notes.SaveIfUnchanged(note.ID, "second", note.UpdatedAt, f.set.Clock.Now())
	if err != nil || conflict || saved.Markdown != "second" {
		t.Fatalf("save on the saved version = %+v, conflict %v, err %v", saved, conflict, err)
	}
	stale, conflict, err := f.notes.SaveIfUnchanged(note.ID, "stale", note.UpdatedAt, f.set.Clock.Now())
	if err != nil || !conflict {
		t.Fatalf("save on an older version: conflict %v, err %v, want a conflict", conflict, err)
	}
	mustEqual(t, "text after the conflict", stale.Markdown, "second")
	got, _ := f.notes.Get(note.ID)
	mustEqual(t, "stored text", got.Markdown, "second")
}

func TestDeleteRemovesOnlyTheNote(t *testing.T) {
	f := newFixture(t)
	keep := f.newNote("keep")
	gone := f.newNote("gone")
	if err := f.notes.Delete(gone.ID); err != nil {
		t.Fatal(err)
	}
	list := f.notes.List()
	mustEqual(t, "notes left", len(list), 1)
	mustEqual(t, "remaining note", list[0].ID, keep.ID)
	if err := f.notes.Delete(gone.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete of a missing note = %v, want ErrNotFound", err)
	}
}

func TestCreateRefusesPastTheNoteLimit(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < MaxNotes; i++ {
		if _, err := f.notes.Create(f.set.Clock.Now()); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err := f.notes.Create(f.set.Clock.Now()); !errors.Is(err, ErrLimit) {
		t.Fatalf("create past the limit = %v, want ErrLimit", err)
	}
}

// Notes with a malformed id are left out of the list and of every lookup.
func TestMalformedNotesAreLeftOut(t *testing.T) {
	f := newFixture(t)
	body := `{"schemaVersion":1,"data":{"notes":[{"id":"bad id","markdown":"x"},{"id":"note-ok","markdown":"kept"}]}}`
	f.set.FS.Seed(filepath.Join(f.dir, notesFile), []byte(body))
	f.restart(false)
	list := f.notes.List()
	mustEqual(t, "notes kept", len(list), 1)
	mustEqual(t, "kept note", list[0].ID, "note-ok")
}

func TestNotesSurviveARestart(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("durable")
	f.restart(false)
	got, ok := f.notes.Get(note.ID)
	if !ok || got.Markdown != "durable" {
		t.Fatalf("after restart = %+v, found %v", got, ok)
	}
}

func mustRecent(t *testing.T, f *fixture) Note {
	t.Helper()
	note, err := f.notes.MostRecentOrCreate(f.set.Clock.Now())
	if err != nil {
		t.Fatalf("most recent: %v", err)
	}
	return note
}

// An imported note keeps its id, text and times; a note whose id the store holds is left as the store has it.
func TestImportKeepsTheNoteAndSkipsAnExistingID(t *testing.T) {
	f := newFixture(t)
	kept := f.newNote("2.0 text")
	old := Note{ID: "note-from-1x", Markdown: "1.x text", CreatedAt: kept.CreatedAt, UpdatedAt: kept.UpdatedAt.Add(-time.Hour)}
	added, err := f.notes.Import(old)
	if err != nil || !added {
		t.Fatalf("import = %v, %v; want the note added", added, err)
	}
	got, ok := f.notes.Get(old.ID)
	if !ok || got.Markdown != "1.x text" || !got.UpdatedAt.Equal(old.UpdatedAt) {
		t.Fatalf("imported note = %+v, found %v", got, ok)
	}
	added, err = f.notes.Import(Note{ID: kept.ID, Markdown: "1.x copy"})
	if err != nil || added {
		t.Fatalf("import over an existing id = %v, %v; want it skipped", added, err)
	}
	if now, _ := f.notes.Get(kept.ID); now.Markdown != "2.0 text" {
		t.Fatalf("existing note = %q, want the store's text", now.Markdown)
	}
	if _, err := f.notes.Import(Note{ID: "Bad ID"}); !errors.Is(err, ErrInvalidNote) {
		t.Fatalf("import with a bad id = %v, want ErrInvalidNote", err)
	}
}
