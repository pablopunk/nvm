package notes

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

var testNow = time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC)

// fakeNotes is an in-memory notes store with the store's rules: the newest note first, and a write checked against the
// saved version.
type fakeNotes struct {
	api.Notes
	notes []api.Note
	seq   int
}

func (f *fakeNotes) List() ([]api.Note, error) {
	out := slices.Clone(f.notes)
	slices.SortStableFunc(out, func(a, b api.Note) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out, nil
}

func (f *fakeNotes) Get(id string) (api.Note, bool, error) {
	i := slices.IndexFunc(f.notes, func(n api.Note) bool { return n.ID == id })
	if i < 0 {
		return api.Note{}, false, nil
	}
	return f.notes[i], true, nil
}

func (f *fakeNotes) Create() (api.Note, error) {
	f.seq++
	note := api.Note{ID: "note-" + string(rune('a'+f.seq)), UpdatedAt: testNow.Add(time.Duration(f.seq) * time.Minute)}
	f.notes = append(f.notes, note)
	return note, nil
}

func (f *fakeNotes) Recent() (api.Note, error) {
	list, _ := f.List()
	if len(list) == 0 {
		return f.Create()
	}
	return list[0], nil
}

func (f *fakeNotes) Update(id, markdown string, updatedAt time.Time) (api.Note, bool, error) {
	for i := range f.notes {
		if f.notes[i].ID != id {
			continue
		}
		if !f.notes[i].UpdatedAt.Equal(updatedAt) {
			return f.notes[i], true, nil
		}
		f.notes[i].Markdown, f.notes[i].UpdatedAt = markdown, testNow.Add(time.Hour)
		return f.notes[i], false, nil
	}
	return api.Note{}, false, errors.New("no such note")
}

func (f *fakeNotes) Delete(id string) error {
	f.notes = slices.DeleteFunc(f.notes, func(n api.Note) bool { return n.ID == id })
	return nil
}

// fakeDrafts records the draft versions the editor committed.
type fakeDrafts struct {
	api.Drafts
	committed []string
}

func (d *fakeDrafts) Commit(key, version string) (bool, error) {
	d.committed = append(d.committed, key+"@"+version)
	return true, nil
}

// fakeWindows records the window actions the built-in asked for.
type fakeWindows struct {
	api.Windows
	created []api.WindowOptions
	toggled []api.WindowRef
}

func (w *fakeWindows) Create(_ api.View, options api.WindowOptions) api.Action {
	w.created = append(w.created, options)
	return api.Action{Kind: api.ActionWindow, Op: "create"}
}

func (w *fakeWindows) Toggle(ref api.WindowRef) api.Action {
	w.toggled = append(w.toggled, ref)
	return api.Action{Kind: api.ActionWindow, Op: "toggle"}
}

// fakeNavigation returns the navigation the host would: the view and whether it was pushed or replaced.
type fakeNavigation struct {
	api.Navigation
}

func (fakeNavigation) Push(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavPush}
}

func (fakeNavigation) Replace(view *api.View) api.Result {
	return api.Result{View: view, Navigation: api.NavReplace}
}

type fakeClock struct{ api.Clock }

func (fakeClock) Now() time.Time { return testNow }

// fakeContext is the context the built-in runs with: the notes, the drafts, the windows and the navigation are fakes.
type fakeContext struct {
	api.Context
	notes   *fakeNotes
	drafts  *fakeDrafts
	windows *fakeWindows
}

func newContext(notes ...api.Note) fakeContext {
	return fakeContext{notes: &fakeNotes{notes: notes}, drafts: &fakeDrafts{}, windows: &fakeWindows{}}
}

func (c fakeContext) Notes() api.Notes           { return c.notes }
func (c fakeContext) Drafts() api.Drafts         { return c.drafts }
func (c fakeContext) Windows() api.Windows       { return c.windows }
func (c fakeContext) Navigation() api.Navigation { return fakeNavigation{} }
func (c fakeContext) Clock() api.Clock           { return fakeClock{} }

func note(id, markdown string, saved time.Time) api.Note {
	return api.Note{ID: id, Markdown: markdown, UpdatedAt: saved}
}

func invocation(id string, args map[string]string) api.Invocation {
	return api.Invocation{Ref: api.ActionRef{ID: id, Args: args}}
}

func TestCommandsKeepTheOneXCommandIDs(t *testing.T) {
	commands, err := New().Commands(newContext())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range commands {
		ids = append(ids, c.ID)
	}
	want := []string{ListCommandID, ToggleCommandID, NewWindowCommandID, NewNoteCommandID}
	if !slices.Equal(ids, want) {
		t.Fatalf("command ids = %v, want %v", ids, want)
	}
}

func TestPaletteListPushesAndStartsWithNewNote(t *testing.T) {
	ctx := newContext(note("note-1", "# Ideas\nship it", testNow))
	result, err := New().RunAction(ctx, invocation(ListCommandID, nil))
	if err != nil {
		t.Fatal(err)
	}
	if result.Navigation != api.NavPush || result.View == nil {
		t.Fatalf("list result = %+v, want a pushed view", result)
	}
	if got := result.View.Items[0].Title; got != newRowTitle {
		t.Fatalf("first row = %q, want %q", got, newRowTitle)
	}
	if got := result.View.Items[1].Title; got != "Ideas" {
		t.Fatalf("note row = %q, want the title Ideas", got)
	}
	if result.View.Items[1].Shortcut != "Control+F" {
		t.Fatalf("palette note row shortcut = %q, want Control+F for Open in Window", result.View.Items[1].Shortcut)
	}
}

func TestWindowListReplacesAndOffersNoOpenInWindow(t *testing.T) {
	ctx := newContext(note("note-1", "Ideas", testNow))
	result, err := New().RunAction(ctx, invocation(refAllNotes, map[string]string{argMode: modeWindow, argNav: navReplace}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Navigation != api.NavReplace {
		t.Fatalf("window list navigation = %q, want replace", result.Navigation)
	}
	for _, action := range result.View.Items[1].Actions {
		if action.Ref != nil && action.Ref.ID == refOpenInWindow {
			t.Fatal("the window's list offers Open in Window")
		}
	}
}

func TestRestoreWindowRebuildsTheMostRecentNote(t *testing.T) {
	ctx := newContext(note("note-1", "# Older", testNow.Add(-time.Hour)), note("note-2", "# Newer", testNow))
	restore, err := New().RestoreWindow(ctx, windowID)
	if err != nil {
		t.Fatal(err)
	}
	if restore == nil || restore.View.Title != "Newer" {
		t.Fatalf("restored view = %+v, want the newest note", restore)
	}
	if restore.Options == nil || !restore.Options.Persistent || restore.Options.RestoreKey != windowID || !restore.Options.RemembersFrame {
		t.Fatalf("restored options = %+v, want a persistent window that remembers its frame", restore.Options)
	}
	if other, err := New().RestoreWindow(ctx, "someone-else"); err != nil || other != nil {
		t.Fatalf("another key restored %+v, %v; want nothing", other, err)
	}
}

// TestFloatingWindowTitlesFollowTheOneXWindows: the toggle and the restore title the window Floating Notes, as 1.x does,
// and a window opened on a new note takes that note's title.
func TestFloatingWindowTitlesFollowTheOneXWindows(t *testing.T) {
	ctx := newContext(note("note-1", "# Ideas", testNow))
	if _, err := New().RunAction(ctx, invocation(ToggleCommandID, nil)); err != nil {
		t.Fatal(err)
	}
	if len(ctx.windows.toggled) != 1 || ctx.windows.toggled[0].Options == nil {
		t.Fatalf("toggled = %+v, want one window with options", ctx.windows.toggled)
	}
	if got := ctx.windows.toggled[0].Options.Title; got != "Floating Notes" {
		t.Fatalf("toggle title = %q, want Floating Notes", got)
	}
	restore, err := New().RestoreWindow(ctx, windowID)
	if err != nil || restore == nil || restore.Options == nil {
		t.Fatalf("restore = %+v, %v; want a window", restore, err)
	}
	if got := restore.Options.Title; got != "Floating Notes" {
		t.Fatalf("restore title = %q, want Floating Notes", got)
	}
	if _, err := New().RunAction(ctx, invocation(NewWindowCommandID, nil)); err != nil {
		t.Fatal(err)
	}
	if len(ctx.windows.created) != 1 || ctx.windows.created[0].Title != "Untitled note" {
		t.Fatalf("created = %+v, want one window titled with the new note", ctx.windows.created)
	}
}

func TestAutosaveWritesTheNoteAndCommitsItsDraft(t *testing.T) {
	ctx := newContext(note("note-1", "old", testNow))
	if _, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refAutosave, Args: map[string]string{argID: "note-1"}}, EditorContent: "new text"}); err != nil {
		t.Fatal(err)
	}
	if ctx.notes.notes[0].Markdown != "new text" {
		t.Fatalf("saved markdown = %q", ctx.notes.notes[0].Markdown)
	}
	if len(ctx.drafts.committed) != 1 || !strings.HasPrefix(ctx.drafts.committed[0], "note-1@") {
		t.Fatalf("committed drafts = %v, want the note's draft at its new version", ctx.drafts.committed)
	}
}

func TestDeleteShownNoteShowsTheNextOne(t *testing.T) {
	ctx := newContext(note("note-1", "first", testNow), note("note-2", "second", testNow.Add(-time.Hour)))
	result, err := New().RunAction(ctx, invocation(refDeleteShown, map[string]string{argID: "note-1", argMode: modePalette}))
	if err != nil {
		t.Fatal(err)
	}
	if result.View == nil || result.View.Editor == nil || result.View.Editor.Draft.Key != "note-2" {
		t.Fatalf("after delete the view = %+v, want the editor on note-2", result.View)
	}
	if result.Toast == nil || result.Toast.Message != "Note deleted" {
		t.Fatalf("toast = %+v", result.Toast)
	}
}

func TestDraftConflictOffersTheChoicesOnTheSavedNote(t *testing.T) {
	ctx := newContext(note("note-1", "saved text", testNow))
	conflict := &api.DraftConflict{Key: "note-1", StoredContent: "typed text", CurrentContent: "saved text"}
	result, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refConflict, Args: map[string]string{argID: "note-1"}}, DraftConflict: conflict})
	if err != nil {
		t.Fatal(err)
	}
	if result.View == nil || len(result.View.Items) != 2 {
		t.Fatalf("conflict view = %+v, want the restore and keep rows", result.View)
	}
	restore := result.View.Items[0].Primary.Ref.Args
	if restore[argText] != "typed text" || restore[argID] != "note-1" {
		t.Fatalf("restore row args = %v, want the unsaved text and the note", restore)
	}
}

func TestRestoreUnsavedWritesTheUnsavedTextAndResolvesTheDraft(t *testing.T) {
	ctx := newContext(note("note-1", "saved text", testNow))
	result, err := New().RunAction(ctx, invocation(api.NoteConflictRestoreRef, map[string]string{argID: "note-1", argText: "typed text", argMode: modeWindow}))
	if err != nil {
		t.Fatal(err)
	}
	if ctx.notes.notes[0].Markdown != "typed text" {
		t.Fatalf("saved markdown = %q, want the unsaved text", ctx.notes.notes[0].Markdown)
	}
	if result.Draft == nil || result.Draft.Resolution != "restore-old" || result.Draft.Key != "note-1" {
		t.Fatalf("draft resolution = %+v", result.Draft)
	}
}

func TestMissingNoteIsReportedNotCreated(t *testing.T) {
	ctx := newContext()
	if _, err := New().RunAction(ctx, invocation(refOpenNote, map[string]string{argID: "gone"})); err == nil {
		t.Fatal("opening a missing note returned no error")
	}
}

func TestCreateAndAutosaveDeclareDataChanges(t *testing.T) {
	ctx := newContext()
	created, err := New().RunAction(ctx, invocation(refNewNote, map[string]string{argMode: modePalette, argNav: navPush}))
	if err != nil || !created.DataChanged || created.View == nil {
		t.Fatalf("create = %+v, %v", created, err)
	}
	notes, err := ctx.Notes().List()
	if err != nil || len(notes) != 1 {
		t.Fatalf("created notes = %+v, %v", notes, err)
	}
	saved, err := New().RunAction(ctx, api.Invocation{Ref: api.ActionRef{ID: refAutosave, Args: map[string]string{argID: notes[0].ID}}, EditorContent: "new title"})
	if err != nil || !saved.DataChanged {
		t.Fatalf("autosave = %+v, %v", saved, err)
	}
}
