package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const typedNote = "Typed before the kill"

// showEditor shows the editor page in a real palette and returns the tester that types into it.
func showEditor(t *testing.T, r *editorRig) *ui.Tester {
	t.Helper()
	p := palette.New(palette.Options{Root: r.editor.page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	tt := ui.NewTester(p.View, 720, 470)
	p.Show()
	tt.Frame()
	return tt
}

// typeIntoEditor shows the editor page in a real palette and types text into it, the way the rig's keyboard does.
func typeIntoEditor(t *testing.T, r *editorRig, text string) {
	t.Helper()
	tt := showEditor(t, r)
	tt.Type(text)
	tt.Frame()
}

// TestEditorAutosavesAKeyTheAreaDidNotReport types the note's first part and lets it autosave, then puts the last key
// into the text without the area reporting a change, as the rig's last keystroke arrived. The draft must hold the note.
func TestEditorAutosavesAKeyTheAreaDidNotReport(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")
	tt := showEditor(t, r)
	tt.Type(typedNote[:len(typedNote)-1])
	tt.Frame()
	r.autosaveTick(defaultDraftDebounce)
	r.editor.text = typedNote
	tt.Frame()
	if !r.editor.draft.dirty {
		t.Fatalf("draft is clean after the unreported key; want it counted as an edit and autosaved")
	}
	r.autosaveTick(defaultDraftDebounce)
	if n := len(drafts.saves); n < 2 || drafts.saves[n-1].Content != typedNote {
		t.Fatalf("draft saves = %+v; want the last one to hold %q", drafts.saves, typedNote)
	}
}

// TestEditorKeepsAnEditThatLandsDuringAWrite autosaves the note's first part while the last key arrives inside the store's
// write, as the UI loop runs during a real write. The write's success must not clear that key's dirty flag, so the
// next autosave writes the whole note.
func TestEditorKeepsAnEditThatLandsDuringAWrite(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")
	tt := showEditor(t, r)
	tt.Type(typedNote[:len(typedNote)-1])
	tt.Frame()
	drafts.duringSave = func() {
		r.editor.text = typedNote
		r.editor.noticeTextChange()
	}
	r.autosaveTick(defaultDraftDebounce)
	if !r.editor.draft.dirty {
		t.Fatalf("draft is clean after a write that a key landed in; want the key kept dirty for the next autosave")
	}
	r.autosaveTick(defaultDraftDebounce)
	if n := len(drafts.saves); n < 2 || drafts.saves[n-1].Content != typedNote {
		t.Fatalf("draft saves = %+v; want the last one to hold %q", drafts.saves, typedNote)
	}
}

// TestEditorReceivesEveryTypedCharacter types the note the rig types and checks that the editor's text holds all of
// it, and that the autosaved draft holds all of it too.
func TestEditorReceivesEveryTypedCharacter(t *testing.T) {
	drafts := newMemDrafts()
	r := openEditor(t, drafts, draftOptions("v1"), "")
	typeIntoEditor(t, r, typedNote)
	if r.editor.text != typedNote {
		t.Fatalf("editor text = %q; want every typed character %q", r.editor.text, typedNote)
	}
	r.autosaveTick(defaultDraftDebounce)
	if n := len(drafts.saves); n == 0 || drafts.saves[n-1].Content != typedNote {
		t.Fatalf("last draft save = %+v; want the full note", drafts.saves)
	}
}
