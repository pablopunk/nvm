package extview

import (
	"errors"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// memDrafts is the host's draft store in memory: one record per key, with a switch that refuses writes.
type memDrafts struct {
	records   map[string]protocol.DraftRecord
	saves     []protocol.DraftSaveArgs
	discarded []string
	saveErr   error
}

func newMemDrafts(records ...protocol.DraftRecord) *memDrafts {
	m := &memDrafts{records: map[string]protocol.DraftRecord{}}
	for _, record := range records {
		m.records[record.Key] = record
	}
	return m
}

func (m *memDrafts) LoadDraft(_ string, key string) (protocol.DraftLookup, error) {
	record, ok := m.records[key]
	return protocol.DraftLookup{Found: ok, Draft: record}, nil
}

func (m *memDrafts) SaveDraft(_ string, args protocol.DraftSaveArgs) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saves = append(m.saves, args)
	m.records[args.Key] = protocol.DraftRecord{Key: args.Key, Version: args.Version, Content: args.Content}
	return nil
}

func (m *memDrafts) DiscardDraft(_ string, key string) error {
	m.discarded = append(m.discarded, key)
	delete(m.records, key)
	return nil
}

// editorRig is one editor view with fakes for the clock, the drafts, the invoke port and the UI thread.
type editorRig struct {
	t       *testing.T
	session *Session
	editor  *editorState
	clock   *fake.Clock
	drafts  *memDrafts
	ui      *uiQueue
	host    *pageHost
	invokes []api.Invocation
	reply   func(api.Invocation) (api.Result, *api.Failure)
}

const draftKey = "note-1"

func openEditor(t *testing.T, drafts *memDrafts, options *api.DraftOptions, text string) *editorRig {
	t.Helper()
	return openEditorReplying(t, drafts, options, text, nil)
}

// openEditorReplying opens an editor whose invoke port answers with reply. The reply is set before the draft is
// attached, so a conflict's handler sees it.
func openEditorReplying(t *testing.T, drafts *memDrafts, options *api.DraftOptions, text string, reply func(api.Invocation) (api.Result, *api.Failure)) *editorRig {
	t.Helper()
	r := &editorRig{t: t, clock: fake.NewClock(), drafts: drafts, ui: newUIQueue(), host: &pageHost{}, reply: reply}
	deps := Deps{
		Clock: r.clock,
		OnUI:  r.ui.post,
		Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			r.invokes = append(r.invokes, in)
			if r.reply == nil {
				done(api.Result{}, nil)
				return
			}
			done(r.reply(in))
		},
	}
	if drafts != nil {
		deps.Drafts = drafts
	}
	r.session = NewSession(deps, "acme.notes", api.View{Kind: api.ViewEditor, Title: "Note"})
	r.editor = newEditorState(r.session, "Note", text, "Write", false)
	if options != nil {
		r.editor.attachDraft(*options)
	}
	return r
}

func draftOptions(version string) *api.DraftOptions {
	return &api.DraftOptions{Key: draftKey, Version: version}
}

// typeText is what the text area does on a frame where the user changed the text.
func (r *editorRig) typeText(text string) {
	r.editor.text = text
	r.editor.edited()
}

// autosaveTick fires the debounce timer and runs the callback it posts on the UI thread.
func (r *editorRig) autosaveTick(debounce time.Duration) {
	r.t.Helper()
	r.clock.AwaitTimerDueIn(debounce)
	r.clock.Advance(debounce)
	r.ui.settle(r.t)
}

func TestEditorRecoversADraftOfTheSameVersion(t *testing.T) {
	drafts := newMemDrafts(protocol.DraftRecord{Key: draftKey, Version: "v1", Content: "typed before the crash"})
	r := openEditor(t, drafts, draftOptions("v1"), "saved text")
	if r.editor.text != "typed before the crash" {
		t.Fatalf("text = %q; want the recovered draft", r.editor.text)
	}
	if r.editor.status != "Recovered unsaved text" || r.editor.draft.dirty {
		t.Fatalf("status = %q, dirty = %v; want a recovered, clean draft", r.editor.status, r.editor.draft.dirty)
	}
}

func TestEditorAutosavesOneDraftAfterTheDebounce(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")

	r.typeText("a")
	r.typeText("ab")
	r.typeText("abc")
	if r.clock.PendingTimers() != 1 {
		t.Fatalf("pending timers = %d; want one debounce timer for a burst of edits", r.clock.PendingTimers())
	}
	r.autosaveTick(defaultDraftDebounce)

	if len(drafts.saves) != 1 || drafts.saves[0].Content != "abc" || drafts.saves[0].Version != "v1" {
		t.Fatalf("saves = %+v; want one draft of the latest text at version v1", drafts.saves)
	}
	if r.editor.draft.dirty || r.editor.status != "Draft saved" {
		t.Fatalf("after the save dirty = %v, status = %q; want clean and saved", r.editor.draft.dirty, r.editor.status)
	}
}

func TestEditorUsesItsOwnDebounceWhenTheViewSetsOne(t *testing.T) {
	drafts := newMemDrafts()
	options := draftOptions("v1")
	options.DebounceMs = 1500
	r := openEditor(t, drafts, options, "")

	r.typeText("slow")
	r.autosaveTick(1500 * time.Millisecond)
	if len(drafts.saves) != 1 {
		t.Fatalf("saves = %d; want the save at the view's own debounce", len(drafts.saves))
	}
}

func TestLeavingTheEditorWritesAPendingDraft(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")

	r.typeText("written on the way out")
	r.editor.leave()
	if len(drafts.saves) != 1 || drafts.saves[0].Content != "written on the way out" {
		t.Fatalf("saves = %+v; want the draft written when the page closes", drafts.saves)
	}
	r.autosaveTick(defaultDraftDebounce)
	if len(drafts.saves) != 1 {
		t.Fatalf("the timer wrote %d drafts after a clean leave; want none more", len(drafts.saves))
	}
}

func TestFailedDraftWriteKeepsTheTextAndRetries(t *testing.T) {
	drafts := newMemDrafts()
	drafts.saveErr = errors.New("disk full")
	r := openEditor(t, drafts, draftOptions("v1"), "")

	r.typeText("must not be lost")
	r.autosaveTick(defaultDraftDebounce)
	if !r.editor.draft.dirty || r.editor.status == "" {
		t.Fatalf("dirty = %v, status = %q; want the text kept dirty with a notice", r.editor.draft.dirty, r.editor.status)
	}

	drafts.saveErr = nil
	r.autosaveTick(defaultDraftDebounce)
	if len(drafts.saves) != 1 || drafts.saves[0].Content != "must not be lost" {
		t.Fatalf("saves = %+v; want the retry to write the text", drafts.saves)
	}
}

func TestWithoutADraftStoreNothingIsScheduled(t *testing.T) {
	r := openEditor(t, nil, draftOptions("v1"), "")
	r.typeText("no store")
	if r.clock.PendingTimers() != 0 || r.editor.draft != nil {
		t.Fatalf("pending timers = %d; want no autosave without a draft store", r.clock.PendingTimers())
	}
}

func TestConflictWithoutAHandlerKeepsTheStoredDraftUntouched(t *testing.T) {
	drafts := newMemDrafts(protocol.DraftRecord{Key: draftKey, Version: "v0", Content: "old draft"})
	r := openEditor(t, drafts, draftOptions("v1"), "current")

	if r.editor.text != "current" || !r.editor.draft.blocked {
		t.Fatalf("text = %q, blocked = %v; want the current text and autosave blocked", r.editor.text, r.editor.draft.blocked)
	}
	r.typeText("typing does not overwrite")
	if r.clock.PendingTimers() != 0 || len(drafts.saves) != 0 {
		t.Fatal("autosave wrote over a conflicting draft")
	}
	if drafts.records[draftKey].Content != "old draft" {
		t.Fatalf("stored draft = %q; want it kept", drafts.records[draftKey].Content)
	}
}

func conflictOptions(version string) *api.DraftOptions {
	options := draftOptions(version)
	options.OnConflict = &api.ActionRef{ID: "resolve"}
	return options
}

func TestConflictResetDiscardsTheStoredDraftAndAutosaveResumes(t *testing.T) {
	drafts := newMemDrafts(protocol.DraftRecord{Key: draftKey, Version: "v0", Content: "old draft"})
	r := openEditorReplying(t, drafts, conflictOptions("v1"), "current", func(api.Invocation) (api.Result, *api.Failure) {
		return api.Result{Draft: &api.DraftResolution{Key: draftKey, Resolution: "reset"}}, nil
	})

	if len(r.invokes) != 1 {
		t.Fatalf("invokes = %d; want the OnConflict handler once", len(r.invokes))
	}
	conflict := r.invokes[0].DraftConflict
	if conflict == nil || conflict.StoredVersion != "v0" || conflict.CurrentVersion != "v1" || conflict.CurrentContent != "current" {
		t.Fatalf("conflict = %+v; want both versions and the current text", conflict)
	}
	if len(drafts.discarded) != 1 || r.editor.draft.blocked {
		t.Fatalf("discarded = %v, blocked = %v; want the draft discarded and autosave resumed", drafts.discarded, r.editor.draft.blocked)
	}
	r.typeText("after reset")
	r.autosaveTick(defaultDraftDebounce)
	if len(drafts.saves) != 1 || drafts.saves[0].Version != "v1" {
		t.Fatalf("saves = %+v; want the draft written at the current version", drafts.saves)
	}
}

func TestConflictRestoreOldShowsTheStoredTextAndWritesItAtTheCurrentVersion(t *testing.T) {
	drafts := newMemDrafts(protocol.DraftRecord{Key: draftKey, Version: "v0", Content: "old draft"})
	r := openEditorReplying(t, drafts, conflictOptions("v1"), "current", func(api.Invocation) (api.Result, *api.Failure) {
		return api.Result{Draft: &api.DraftResolution{Key: draftKey, Resolution: "restore-old"}}, nil
	})

	if r.editor.text != "old draft" {
		t.Fatalf("text = %q; want the stored draft after restore-old", r.editor.text)
	}
	r.autosaveTick(defaultDraftDebounce)
	if len(drafts.saves) != 1 || drafts.saves[0].Content != "old draft" || drafts.saves[0].Version != "v1" {
		t.Fatalf("saves = %+v; want the restored text written at v1", drafts.saves)
	}
}

func TestConflictMigrateUsesTheHandlersContent(t *testing.T) {
	drafts := newMemDrafts(protocol.DraftRecord{Key: draftKey, Version: "v0", Content: "old draft"})
	r := openEditorReplying(t, drafts, conflictOptions("v1"), "current", func(api.Invocation) (api.Result, *api.Failure) {
		return api.Result{Draft: &api.DraftResolution{Key: draftKey, Resolution: "migrate", Content: "merged"}}, nil
	})
	if r.editor.text != "merged" {
		t.Fatalf("text = %q; want the migrated content", r.editor.text)
	}
}

func TestConflictHandlerFailureLeavesTheDraftBlocked(t *testing.T) {
	drafts := newMemDrafts(protocol.DraftRecord{Key: draftKey, Version: "v0", Content: "old draft"})
	r := openEditorReplying(t, drafts, conflictOptions("v1"), "current", func(api.Invocation) (api.Result, *api.Failure) {
		return api.Result{}, &api.Failure{Code: api.CodeInternal, Message: "the handler crashed"}
	})
	if !r.editor.draft.blocked || r.editor.status != "the handler crashed" {
		t.Fatalf("blocked = %v, status = %q; want the draft still blocked with the failure", r.editor.draft.blocked, r.editor.status)
	}
}

func TestSuccessfulSubmitClearsTheDraftWhenNothingChangedDuringTheRun(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")
	r.typeText("final text")
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "save"}}

	r.editor.submitTo(r.host, &action, "final text")
	if r.editor.draft.dirty {
		t.Fatal("a successful submit left the draft dirty; the extension commits it")
	}
	if len(r.invokes) != 1 || r.invokes[0].EditorContent != "final text" || r.invokes[0].Ref.ID != "save" {
		t.Fatalf("invokes = %+v; want the submit with the editor text", r.invokes)
	}
}

func TestEditsDuringASubmitKeepTheDraftDirty(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")
	r.typeText("first")
	r.reply = func(api.Invocation) (api.Result, *api.Failure) {
		r.typeText("typed while saving")
		return api.Result{}, nil
	}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "save"}}

	r.editor.submitTo(r.host, &action, "first")
	if !r.editor.draft.dirty {
		t.Fatal("text typed during the submit was marked as saved")
	}
}

func TestFailedSubmitKeepsTheDraftDirty(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")
	r.typeText("keep me")
	r.reply = func(api.Invocation) (api.Result, *api.Failure) {
		return api.Result{}, &api.Failure{Code: api.CodeInternal, Message: "save failed"}
	}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "save"}}

	r.editor.submitTo(r.host, &action, "keep me")
	if !r.editor.draft.dirty || len(r.host.toasts) != 1 || r.host.toasts[0] != "save failed" {
		t.Fatalf("dirty = %v, toasts = %v; want the text kept and the failure shown", r.editor.draft.dirty, r.host.toasts)
	}
}

func TestEditorSubmitWithoutAnActionSaysSo(t *testing.T) {
	r := openEditor(t, nil, nil, "text")
	r.editor.submitTo(r.host, nil, "text")
	if len(r.invokes) != 0 || len(r.host.toasts) != 1 {
		t.Fatalf("invokes = %d, toasts = %v; want one toast and no run", len(r.invokes), r.host.toasts)
	}
}
