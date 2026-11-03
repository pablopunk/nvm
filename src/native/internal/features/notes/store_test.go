package notes

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Ported from src/app/extensions/floating-notes.test.ts, case "Floating Notes command toggles a persistent
// floating window with an autosaving editor": the command opens the most recent note (or creates one), its
// editor autosaves on the 400 ms debounce, and reopening the command keeps the same note.
func TestToggleOpensTheMostRecentNoteOrCreatesOne(t *testing.T) {
	f := newFixture(t)
	mustEqual(t, "autosave debounce (ms)", AutosaveDelay.Milliseconds(), int64(400))
	first, err := f.notes.MostRecentOrCreate(f.set.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	sess := f.mustSession(first.ID)
	mustEqual(t, "draft base is the saved version", sess.base, first.Version())
	mustEqual(t, "notes after the first toggle", len(f.notes.List()), 1)

	again, err := f.notes.MostRecentOrCreate(f.set.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, "reopened note", again.ID, first.ID)
	mustEqual(t, "notes after reopening", len(f.notes.List()), 1)
}

// Ported from the same test file, case "autosave writes content, bumps the version, and commits the draft". The
// 1.x host kept the draft record and advanced its version; this store removes the draft once the note holds the
// same text, which gives the same result on the next open: nothing to recover and nothing to resolve.
func TestAutosaveWritesTheNoteBumpsItsVersionAndClearsTheDraft(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("")
	sess := f.mustSession(note.ID)
	autosaved := make(chan struct{}, 4)
	sess.onSaved = func() { autosaved <- struct{}{} }

	if err := sess.Edit("# A durable idea"); err != nil {
		t.Fatal(err)
	}
	mustEqual(t, "draft count before autosave", len(f.notes.Drafts()), 1)
	f.set.Clock.Advance(AutosaveDelay)
	f.awaitSignal(autosaved, "autosave")

	saved, _ := f.notes.Get(note.ID)
	mustEqual(t, "saved markdown", saved.Markdown, "# A durable idea")
	mustEqual(t, "title", saved.Title(), "A durable idea")
	if saved.Version() <= note.Version() {
		t.Fatalf("version %d did not advance past %d", saved.Version(), note.Version())
	}
	mustEqual(t, "draft count after autosave", len(f.notes.Drafts()), 0)
	mustEqual(t, "status after autosave", sess.Status(), StatusSaved)
}

// Ported from the same test file, case "conflict action offers restore and reset resolutions that keep versions
// consistent". The two choices are "Keep draft" (the unsaved text becomes the note) and "Keep saved version"
// (the saved text is shown and the draft is dropped).
func TestConflictOffersKeepDraftAndKeepSavedAsResolutions(t *testing.T) {
	setup := func(t *testing.T) (*fixture, Note) {
		f := newFixture(t)
		note := f.newNote("saved text")
		sess := f.mustSession(note.ID)
		if err := sess.Edit("unsaved work"); err != nil {
			t.Fatal(err)
		}
		f.advanceAwaitingWrite(DraftDelay, draftsFile)
		f.changeSavedElsewhere(note.ID, "changed elsewhere")
		f.restart(false)
		return f, note
	}

	t.Run("keep saved version", func(t *testing.T) {
		f, note := setup(t)
		sess := f.mustSession(note.ID)
		conflict, ok := sess.Conflict()
		if !ok {
			t.Fatal("no conflict for a draft on an older version")
		}
		mustEqual(t, "conflict saved", conflict.Saved.Markdown, "changed elsewhere")
		mustEqual(t, "conflict draft", conflict.Draft.Content, "unsaved work")
		if err := sess.Edit("typing blocked"); !errors.Is(err, ErrConflictPending) {
			t.Fatalf("edit during conflict = %v, want ErrConflictPending", err)
		}
		if err := sess.KeepSaved(); err != nil {
			t.Fatal(err)
		}
		mustEqual(t, "text after keep saved", sess.Text(), "changed elsewhere")
		mustEqual(t, "drafts after keep saved", len(f.notes.Drafts()), 0)
	})

	t.Run("keep draft", func(t *testing.T) {
		f, note := setup(t)
		autosaved := make(chan struct{}, 4)
		sess, err := f.notes.OpenSession(note.ID, func() { autosaved <- struct{}{} })
		if err != nil {
			t.Fatal(err)
		}
		if err := sess.KeepDraft(); err != nil {
			t.Fatal(err)
		}
		mustEqual(t, "text after keep draft", sess.Text(), "unsaved work")
		f.set.Clock.Advance(AutosaveDelay)
		f.awaitSignal(autosaved, "autosave after keep draft")
		saved, _ := f.notes.Get(note.ID)
		mustEqual(t, "saved after keep draft", saved.Markdown, "unsaved work")
	})
}

// Ported from the same test file, case "collection lists notes with CRUD operations and deletes discard
// drafts": notes list newest first, and deleting one discards its draft and leaves the others.
func TestDeleteRemovesTheNoteAndItsDraftAndTheListStaysOrdered(t *testing.T) {
	f := newFixture(t)
	older := f.newNote("older")
	f.set.Clock.Advance(time.Minute)
	newer := f.newNote("newer")
	mustEqual(t, "list order first", f.notes.List()[0].ID, newer.ID)

	sess := f.mustSession(older.ID)
	if err := sess.Edit("older, edited"); err != nil {
		t.Fatal(err)
	}
	mustEqual(t, "drafts before delete", len(f.notes.Drafts()), 1)

	if err := f.notes.Delete(older.ID); err != nil {
		t.Fatal(err)
	}
	mustEqual(t, "notes after delete", len(f.notes.List()), 1)
	mustEqual(t, "drafts after delete", len(f.notes.Drafts()), 0)
	if err := f.notes.Delete(older.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
}

// Ported from the same test file, case "deleting the open note moves to the next note or a fresh one": the
// remaining notes stay, and once none is left the command creates a fresh one.
func TestDeletingTheOpenNoteLeavesTheOthersOrAFreshOne(t *testing.T) {
	f := newFixture(t)
	first := f.newNote("first")
	f.set.Clock.Advance(time.Minute)
	second := f.newNote("second")
	if err := f.notes.Delete(second.ID); err != nil {
		t.Fatal(err)
	}
	mustEqual(t, "remaining note", f.notes.List()[0].ID, first.ID)
	if err := f.notes.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	fresh, err := f.notes.MostRecentOrCreate(f.set.Clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == first.ID || fresh.ID == second.ID {
		t.Fatalf("fresh note reuses a deleted id: %s", fresh.ID)
	}
}

// Ported from the same test file, case "restoreWindow rebuilds the floating window with the most recent note":
// the floating state and the last frame survive a restart, and the frame is kept when the window closes.
func TestFloatingNotesAndTheirFramesSurviveARestart(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("floating")
	mustEqual(t, "floating before any", len(f.notes.Floating()), 0)
	if err := f.notes.SetFloating(note.ID, true); err != nil {
		t.Fatal(err)
	}
	frame := Frame{X: 10, Y: 20, Width: 300, Height: 200}
	if err := f.notes.SetFrame(note.ID, frame); err != nil {
		t.Fatal(err)
	}
	if err := f.st.Flush(); err != nil {
		t.Fatal(err)
	}
	f.restart(false)
	floating := f.notes.Floating()
	mustEqual(t, "floating after restart", len(floating), 1)
	if floating[0].Frame == nil || *floating[0].Frame != frame {
		t.Fatalf("frame after restart = %+v, want %+v", floating[0].Frame, frame)
	}
	if err := f.notes.SetFloating(note.ID, false); err != nil {
		t.Fatal(err)
	}
	closed, _ := f.notes.Get(note.ID)
	if closed.Floating || closed.Frame == nil {
		t.Fatalf("closed window kept floating=%v frame=%v", closed.Floating, closed.Frame)
	}
}

// Ported from extension-draft-store.test.ts, case "recovers matching owner-scoped drafts and keeps owners
// isolated": a draft recovers only for the note it belongs to.
func TestDraftRecoveryIsScopedToItsNote(t *testing.T) {
	f := newFixture(t)
	a := f.newNote("saved a")
	b := f.newNote("saved b")
	sessA := f.mustSession(a.ID)
	if err := sessA.Edit("draft a"); err != nil {
		t.Fatal(err)
	}
	f.advanceAwaitingWrite(DraftDelay, draftsFile)
	f.restart(false)

	recovered := f.mustSession(a.ID)
	mustEqual(t, "a recovers its draft", recovered.Text(), "draft a")
	other := f.mustSession(b.ID)
	mustEqual(t, "b keeps its saved text", other.Text(), "saved b")
	if _, ok := other.Conflict(); ok {
		t.Fatal("b reports a conflict for a's draft")
	}
}

// Ported from the same test file, case "resolves a version conflict atomically and rejects replay or
// retargeting": a resolution applies once, a resolved conflict cannot be replayed, and a stale conflict is
// refreshed rather than applied.
func TestConflictResolutionAppliesOnceAndRefusesStaleVersions(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("old")
	sess := f.mustSession(note.ID)
	if err := sess.Edit("new"); err != nil {
		t.Fatal(err)
	}
	f.advanceAwaitingWrite(DraftDelay, draftsFile)
	f.changeSavedElsewhere(note.ID, "moved on")
	f.restart(false)

	conflicted := f.mustSession(note.ID)
	f.changeSavedElsewhere(note.ID, "moved on again")
	if err := conflicted.KeepDraft(); !errors.Is(err, ErrStaleConflict) {
		t.Fatalf("keep draft on a stale conflict = %v, want ErrStaleConflict", err)
	}
	refreshed, ok := conflicted.Conflict()
	if !ok || refreshed.Saved.Markdown != "moved on again" {
		t.Fatalf("conflict not refreshed: %+v (ok %v)", refreshed.Saved, ok)
	}
	if err := conflicted.KeepSaved(); err != nil {
		t.Fatal(err)
	}
	if err := conflicted.KeepSaved(); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("replayed resolution = %v, want ErrNoConflict", err)
	}
	if err := conflicted.KeepDraft(); !errors.Is(err, ErrNoConflict) {
		t.Fatalf("resolution after resolving = %v, want ErrNoConflict", err)
	}
}

// Ported from the same test file, case "rejects malformed bounded draft inputs and purges all owner state": a
// draft needs a well-formed note id, a non-negative version and text within the limit.
func TestRejectsMalformedDraftsAndDeletingANoteRemovesOnlyItsDraft(t *testing.T) {
	f := newFixture(t)
	a := f.newNote("a")
	b := f.newNote("b")
	bad := []Draft{
		{NoteID: "", Base: 1, Content: "bad"},
		{NoteID: "../escape", Base: 1, Content: "bad"},
		{NoteID: a.ID, Base: -1, Content: "bad"},
		{NoteID: a.ID, Base: 1, Content: strings.Repeat("x", MaxTextBytes+1)},
	}
	for _, d := range bad {
		if err := f.notes.putDraft(d); !errors.Is(err, ErrInvalidDraft) {
			t.Fatalf("putDraft(%q) = %v, want ErrInvalidDraft", d.NoteID, err)
		}
	}
	mustEqual(t, "drafts after rejected writes", len(f.notes.Drafts()), 0)

	if err := f.notes.putDraft(Draft{NoteID: a.ID, Base: a.Version(), Content: "a draft"}); err != nil {
		t.Fatal(err)
	}
	if err := f.notes.putDraft(Draft{NoteID: b.ID, Base: b.Version(), Content: "b draft"}); err != nil {
		t.Fatal(err)
	}
	if err := f.notes.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	drafts := f.notes.Drafts()
	mustEqual(t, "drafts left", len(drafts), 1)
	mustEqual(t, "draft kept belongs to b", drafts[0].NoteID, b.ID)
}

// Ported from the same test file, case "commit advances the stored version so the next open recovers": after a
// save the next open shows the saved text with nothing to recover.
func TestAfterAutosaveTheNextOpenHasNothingToRecover(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("base")
	sess := f.mustSession(note.ID)
	autosaved := make(chan struct{}, 4)
	sess.onSaved = func() { autosaved <- struct{}{} }
	if err := sess.Edit("base, typed"); err != nil {
		t.Fatal(err)
	}
	f.set.Clock.Advance(AutosaveDelay)
	f.awaitSignal(autosaved, "autosave")
	f.restart(false)

	reopened := f.mustSession(note.ID)
	mustEqual(t, "text", reopened.Text(), "base, typed")
	mustEqual(t, "recovered", reopened.Recovered(), false)
	if _, ok := reopened.Conflict(); ok {
		t.Fatal("conflict after a clean save")
	}
}

// Ported from the same test file, case "hydrate restores persisted records and skips malformed entries": the
// drafts file keeps well-formed records and skips the rest.
func TestLoadKeepsWellFormedDraftsAndSkipsMalformedOnes(t *testing.T) {
	f := newFixture(t)
	note := f.newNote("saved")
	body := `{"schemaVersion":1,"data":{"drafts":[` +
		`{"noteId":"` + note.ID + `","base":` + strconv.FormatInt(note.Version(), 10) + `,"content":"persisted"},` +
		`{"noteId":"","base":1,"content":"malformed"}]}}`
	f.set.FS.Seed(filepath.Join(f.dir, draftsFile), []byte(body))
	f.restart(false)

	mustEqual(t, "drafts kept", len(f.notes.Drafts()), 1)
	sess := f.mustSession(note.ID)
	mustEqual(t, "recovered text", sess.Text(), "persisted")
}
