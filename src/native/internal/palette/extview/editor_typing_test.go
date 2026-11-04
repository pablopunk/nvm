package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const typedNote = "Typed before the kill"

// typeIntoEditor shows the editor page in a real palette and types text into it, the way the rig's keyboard does.
func typeIntoEditor(t *testing.T, r *editorRig, text string) {
	t.Helper()
	p := palette.New(palette.Options{Root: r.editor.page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	tt := ui.NewTester(p.View, 720, 470)
	p.Show()
	tt.Frame()
	tt.Type(text)
	tt.Frame()
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
