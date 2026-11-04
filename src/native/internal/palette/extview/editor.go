package extview

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// editorState is one editor page: its text, the draft it autosaves under when the view asks for one, and what its
// submit does with the text.
type editorState struct {
	sess        *Session
	page        *palette.View
	text        string
	placeholder string
	readOnly    bool
	status      string
	submitFn    func(h search.Host, text string)
	draft       *draftTrack
	edits       uint64
}

// draftTrack is the autosave state of one editor. Blocked means a stored draft from another version waits for the
// user's choice, so autosave must not overwrite it yet.
type draftTrack struct {
	options api.DraftOptions
	dirty   bool
	armed   bool
	blocked bool
}

// editorView builds the page of a ViewEditor. Its submit action gets the text as Invocation.EditorContent.
func (s *Session) editorView(spec api.EditorSpec) *palette.View {
	ed := newEditorState(s, s.view.Title, spec.Content, spec.Placeholder, spec.ReadOnly)
	ed.submitFn = func(h search.Host, text string) { ed.submitTo(h, spec.Submit, text) }
	if spec.Draft != nil {
		ed.attachDraft(*spec.Draft)
	}
	return ed.page
}

// multilineEditor is the editor a multi-line form field opens. Save pops it and hands the text to done, which stores
// the value on the form page below.
func (s *Session) multilineEditor(field api.Field, text string, done func(h search.Host, text string)) *editorState {
	ed := newEditorState(s, fieldName(field), text, fieldPlaceholder(field), false)
	ed.submitFn = func(h search.Host, text string) {
		h.Pop()
		done(h, text)
	}
	return ed
}

func newEditorState(s *Session, title, text, placeholder string, readOnly bool) *editorState {
	ed := &editorState{sess: s, text: text, placeholder: placeholder, readOnly: readOnly}
	ed.page = &palette.View{
		Title: title, HideInput: true, Hint: editorHint(),
		OnKey:   ed.key,
		OnLeave: ed.leave,
		Body:    ed.body,
	}
	return ed
}

// body draws the text area and the status line under it. A change of text is noticed here, the way the notes editor does.
func (ed *editorState) body(c *ui.Context, _ search.Host) {
	t := design.Tokens(c)
	ui.Column(c).Grow(1).Children(func() {
		area := ui.TextArea(c, &ed.text).Grow(1).Margin(12).Padding(10).FontSize(14).Placeholder(ed.placeholder).ReadOnly(ed.readOnly).AutoFocus()
		if area.Changed() {
			ed.edited()
		}
		ui.Text(c, ed.statusLine()).FontSize(11).TextColor(t.Tertiary).Margin(12)
	})
}

func (ed *editorState) statusLine() string {
	if ed.status == "" && ed.draft != nil && ed.draft.dirty {
		return "Unsaved changes"
	}
	return ed.status
}

// key submits on Command-Enter. Escape is left to the palette, which goes back and saves a dirty draft on the way.
func (ed *editorState) key(c *ui.Context, h search.Host) bool {
	if ed.readOnly || !c.Shortcut(palette.Mod, ui.KeyEnter) {
		return false
	}
	ed.submitFn(h, ed.text)
	return true
}

// leave writes a pending draft before the page goes away, so leaving never loses typed text.
func (ed *editorState) leave() {
	ed.flush()
}

// submitTo runs the view's submit action with the text. A successful run clears the draft's dirty flag unless the
// text changed while the run was in flight; the extension is expected to commit its own draft.
func (ed *editorState) submitTo(h search.Host, action *api.Action, text string) {
	if action == nil || action.Ref == nil {
		h.Toast(unavailableMessage)
		return
	}
	edits := ed.edits
	ed.sess.submit(h, api.Invocation{Ref: *action.Ref, EditorContent: text}, func(failed bool) {
		if failed || ed.draft == nil || ed.edits != edits {
			return
		}
		ed.draft.dirty = false
	})
}

func (ed *editorState) edited() {
	ed.edits++
	ed.status = ""
	if ed.draft == nil {
		return
	}
	ed.draft.dirty = true
	ed.armTimer()
}

// armTimer schedules one flush after the debounce. It never has two timers pending, and it stands still while blocked.
func (ed *editorState) armTimer() {
	track, clock := ed.draft, ed.sess.deps.Clock
	if track == nil || track.armed || track.blocked || clock == nil {
		return
	}
	track.armed = true
	timer := clock.After(durationMs(track.options.DebounceMs, int64(defaultDraftDebounce/time.Millisecond)))
	go func() {
		<-timer
		ed.sess.onUI(func() {
			track.armed = false
			ed.flush()
		})
	}()
}

// flush writes the text as the draft, if it has unsaved changes and nothing blocks the write. A failed write keeps the
// text dirty and retries on the next debounce.
func (ed *editorState) flush() {
	track, store := ed.draft, ed.sess.deps.Drafts
	if track == nil || !track.dirty || track.blocked || store == nil {
		return
	}
	args := protocol.DraftSaveArgs{Key: track.options.Key, Version: track.options.Version, Content: ed.text}
	if err := store.SaveDraft(ed.sess.extID, args); err != nil {
		ed.status = "The draft could not be saved; retrying"
		ed.armTimer()
		return
	}
	track.dirty = false
	ed.status = "Draft saved"
}

// attachDraft recovers the stored draft of the view. A draft of the same version is the text the user typed, so it
// comes back; one of another version is a conflict, which the extension resolves.
func (ed *editorState) attachDraft(options api.DraftOptions) {
	store := ed.sess.deps.Drafts
	if store == nil {
		return
	}
	ed.draft = &draftTrack{options: options}
	lookup, err := store.LoadDraft(ed.sess.extID, options.Key)
	switch {
	case err != nil:
		ed.draft.blocked = true
		ed.status = "The draft could not be read; it will not be overwritten"
	case !lookup.Found:
	case lookup.Draft.Version == options.Version:
		ed.text = lookup.Draft.Content
		ed.status = "Recovered unsaved text"
	default:
		ed.resolveConflict(lookup.Draft)
	}
}

// resolveConflict asks the extension's OnConflict handler what to do with a stored draft of another version. Without
// a handler the draft is kept as it is: autosave stays blocked, so nothing is overwritten silently.
func (ed *editorState) resolveConflict(stored protocol.DraftRecord) {
	track := ed.draft
	track.blocked = true
	if track.options.OnConflict == nil || ed.sess.deps.Invoke == nil {
		ed.status = "An unsaved draft from another version is kept until you choose"
		return
	}
	conflict := api.DraftConflict{
		Key: track.options.Key, StoredVersion: stored.Version, StoredContent: stored.Content,
		CurrentVersion: track.options.Version, CurrentContent: ed.text,
	}
	in := api.Invocation{Ref: *track.options.OnConflict, DraftConflict: &conflict}
	ed.sess.deps.Invoke(ed.sess.extID, in, func(result api.Result, failure *api.Failure) {
		ed.applyConflict(result, failure, stored)
	})
}

// applyConflict applies the resolution the handler chose: reset discards the stored draft and keeps the current text,
// restore-old shows the stored text, and migrate shows the handler's content. Each one lets autosave write again.
func (ed *editorState) applyConflict(result api.Result, failure *api.Failure, stored protocol.DraftRecord) {
	track, store := ed.draft, ed.sess.deps.Drafts
	if failure != nil {
		ed.status = failure.Message
		return
	}
	if result.Draft == nil {
		ed.status = "The draft conflict was not resolved"
		return
	}
	switch result.Draft.Resolution {
	case "reset":
		if err := store.DiscardDraft(ed.sess.extID, track.options.Key); err != nil {
			ed.status = "The stored draft could not be discarded"
			return
		}
	case "restore-old":
		ed.text = stored.Content
		track.dirty = true
	case "migrate":
		ed.text = result.Draft.Content
		track.dirty = true
	default:
		ed.status = "The draft conflict has an unknown resolution"
		return
	}
	track.blocked = false
	ed.status = ""
	ed.armTimer()
}

// editorHint is the footer of an editor: the submit shortcut in the palette's modifier, then Back.
func editorHint() string {
	return saveHint() + "   esc Back"
}

func saveHint() string {
	return palette.KeyLabel("⌃↵") + " Save"
}
