package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// The 1.x fixture: one note the 2.0 store lacks, one whose id the store holds, and one with an id the store cannot take.
const legacyNotesFixture = `{"notes":[
 {"id":"note-1x-old","content":"Old one\nmore","createdAt":1700000000000,"updatedAt":1700000100000},
 {"id":"note-dup","content":"1.x copy","createdAt":1,"updatedAt":2},
 {"id":"Bad ID","content":"skipped","createdAt":1,"updatedAt":2}]}`

// TestImportLegacyNotesBringsTheNotesInOnce runs the import over a fixture on a fake disk: the 1.x note is added with
// its times, the note the store holds keeps its text, and a note deleted after the import stays deleted.
func TestImportLegacyNotesBringsTheNotesInOnce(t *testing.T) {
	set := fake.NewSet()
	legacy := "/1x/extension-storage/nevermind.floating-notes.json"
	set.FS.Seed(legacy, []byte(legacyNotesFixture))
	set.FS.Seed("/notes/notes.json", goldenEnvelope(t, map[string]any{"notes": []notes.Note{
		{ID: "note-dup", Markdown: "2.0 text", CreatedAt: goldenNow, UpdatedAt: goldenNow},
	}}))
	st, err := store.Open(set.FS, set.Clock, "/notes")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ns := notes.New(st, set.Clock, false)
	marker := store.Doc(st, legacyNotesMarkerFile, 1, func() legacyNotesMarker { return legacyNotesMarker{} })

	importLegacyNotes(set.FS, legacy, ns, marker, goldenNow)

	old, ok := ns.Get("note-1x-old")
	if !ok || old.Markdown != "Old one\nmore" || !old.UpdatedAt.Equal(time.UnixMilli(1700000100000).UTC()) {
		t.Fatalf("imported note = %+v, found %v", old, ok)
	}
	if dup, _ := ns.Get("note-dup"); dup.Markdown != "2.0 text" {
		t.Fatalf("existing note = %q, want the 2.0 text", dup.Markdown)
	}
	if _, ok := ns.Get("Bad ID"); ok {
		t.Fatal("a note with a bad id was imported")
	}
	if _, err := set.FS.Stat(marker.Path()); err != nil {
		t.Fatalf("the import marker was not written: %v", err)
	}

	if err := ns.Delete("note-1x-old"); err != nil {
		t.Fatal(err)
	}
	importLegacyNotes(set.FS, legacy, ns, marker, goldenNow)
	if _, ok := ns.Get("note-1x-old"); ok {
		t.Fatal("a deleted 1.x note came back on the next launch")
	}
}
