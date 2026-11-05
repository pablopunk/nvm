package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// TestEditorRunsTheAutosaveActionWithTheDraftText checks the Autosave action of a draft gets the text the draft holds,
// once the debounce writes it. The extension's store takes the same text from there.
func TestEditorRunsTheAutosaveActionWithTheDraftText(t *testing.T) {
	options := draftOptions("v1")
	options.Autosave = &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "save-note"}}
	r := openEditor(t, newMemDrafts(), options, "")
	r.typeText("Saved by autosave")
	r.autosaveTick(defaultDraftDebounce)
	last := r.invokes[len(r.invokes)-1]
	if last.Ref.ID != "save-note" || last.EditorContent != "Saved by autosave" {
		t.Fatalf("last invocation = %+v, want the autosave action with the draft text", last)
	}
}

// TestMarkdownSourceKeepsTheNoteMarkdown checks the rich document of a Markdown editor reads back the Markdown it was
// made from, and is remade when the note's text changes under it.
func TestMarkdownSourceKeepsTheNoteMarkdown(t *testing.T) {
	var source design.MarkdownSource
	const note = "# Heading\n\nNote body text"
	if got := source.Sync(note).Text().Markdown(); got != note {
		t.Fatalf("document Markdown = %q, want %q", got, note)
	}
	if got := source.Sync("Replaced").Text().Markdown(); got != "Replaced" {
		t.Fatalf("document after the text changed = %q, want the new text", got)
	}
}

// TestMarkdownEditorPageDrawsItsChrome is an L2 test: a Markdown editor page draws its title and footer in a palette,
// without a native window. The rich text itself is not in the tester's text, so its drawing is checked by the goldens.
func TestMarkdownEditorPageDrawsItsChrome(t *testing.T) {
	view := api.View{ID: "note", Kind: api.ViewEditor, Title: "Note",
		Editor: &api.EditorSpec{Content: "# Heading\n\nNote body text", Format: "markdown"}}
	page := NewSession(Deps{}, "notes", view).Open(&fakeHost{})
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true})
	p.Attach(editWindow{})
	tt := ui.NewTester(p.View, 560, 420)
	p.Show()
	tt.Frame()
	if !tt.HasText("Note") || !tt.HasText(palette.KeyLabel("⌃↵ Save   esc Back")) {
		t.Fatalf("the Markdown editor page draws %q, want its title and footer", tt.Texts())
	}
}

// TestMarkdownEditorPageNamesItsEditorAsInput checks a Markdown editor page hands the keyboard to its editor: the page
// hides its search field, so the palette focuses the input the page names.
func TestMarkdownEditorPageNamesItsEditorAsInput(t *testing.T) {
	view := api.View{ID: "note", Kind: api.ViewEditor, Title: "Note",
		Editor: &api.EditorSpec{Content: "Note body", Format: "markdown"}}
	page := NewSession(Deps{}, "notes", view).Open(&fakeHost{})
	if page.Input == nil || !page.HideInput {
		t.Fatalf("Markdown editor page Input = %v, HideInput = %v; want the page to name its editor as its input", page.Input, page.HideInput)
	}
}
