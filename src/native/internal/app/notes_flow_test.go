package app

import (
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Keyboard flows through the palette's navigation stack, as the keys drive it: rows are chosen and their actions run
// through the same path Enter takes. Actions that hide the palette are not run here (the goldens have no window);
// they are checked by the views they lead to and by the functions they call.

func findRow(t *testing.T, p *App, title string) search.Item {
	t.Helper()
	for _, it := range p.Results() {
		if it.Title == title {
			return it
		}
	}
	t.Fatalf("no row %q among %d rows", title, len(p.Results()))
	return search.Item{}
}

// The first row of the list starts a note: its action opens the editor above the list, and the list stays below it.
func TestNewNoteRowOpensTheEditorAboveTheList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.notesView())
	before := len(p.notes.List())

	goldenChoose(t, p, 0, "Create")

	if top := topView(p); top.Title != "Note" {
		t.Fatalf("top view = %q, want the note editor", top.Title)
	}
	if got := len(p.notes.List()); got != before+1 {
		t.Fatalf("notes = %d, want %d", got, before+1)
	}
	p.Pop()
	if top := topView(p); top.Title != "Floating Notes" {
		t.Fatalf("after Escape the top view is %q, want the list", top.Title)
	}
}

// Deleting asks first: the confirmation is on top, Cancel returns to the list and keeps the note, and only the
// confirmed delete removes it.
func TestDeleteAsksForConfirmationAndCancelKeepsTheNote(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.notesView())
	list := p.notes.List()
	target := list[0]

	p.Push(p.confirmDeleteNote(target.ID, target.Title()))
	if top := topView(p); top.Title != "Confirm" {
		t.Fatalf("top view = %q, want the confirmation", top.Title)
	}
	goldenChoose(t, p, 1, "Cancel")
	if top := topView(p); top.Title != "Floating Notes" {
		t.Fatalf("after Cancel the top view is %q, want the list", top.Title)
	}
	if _, ok := p.notes.Get(target.ID); !ok {
		t.Fatal("cancelled delete removed the note")
	}

	p.deleteNote(target.ID)
	if _, ok := p.notes.Get(target.ID); ok {
		t.Fatal("confirmed delete kept the note")
	}
}

// Keep draft: the choice closes the choices, the editor shows the unsaved text, and the saved note is unchanged
// until autosave.
func TestKeepDraftReturnsToTheEditorWithTheUnsavedText(t *testing.T) {
	p := goldenPaletteWith(t, true)
	view, edit := p.noteEditorView("note-ideas")
	goldenOpen(p, view)
	p.Push(p.conflictView(edit))

	goldenChoose(t, p, 1, "Restore unsaved changes")

	if top := topView(p); top.Title != "Note" {
		t.Fatalf("top view = %q, want the editor", top.Title)
	}
	if !strings.Contains(edit.text, "sketched on the train") {
		t.Fatalf("editor text = %q, want the unsaved text", edit.text)
	}
	if _, pending := edit.sess.Conflict(); pending {
		t.Fatal("conflict still pending after Keep draft")
	}
}

// Keep saved version: the editor shows the saved text and the draft is dropped.
func TestKeepSavedVersionShowsTheSavedText(t *testing.T) {
	p := goldenPaletteWith(t, true)
	view, edit := p.noteEditorView("note-ideas")
	goldenOpen(p, view)
	p.Push(p.conflictView(edit))

	goldenChoose(t, p, 0, "Keep saved version")

	saved, _ := p.notes.Get("note-ideas")
	if edit.text != saved.Markdown {
		t.Fatalf("editor text = %q, want the saved text %q", edit.text, saved.Markdown)
	}
	if len(p.notes.Drafts()) != 0 {
		t.Fatal("the draft survived Keep saved version")
	}
}

// Compare opens the diff over the choices; Escape returns to them.
func TestCompareOpensTheDiffAndEscapeReturnsToTheChoices(t *testing.T) {
	p := goldenPaletteWith(t, true)
	view, edit := p.noteEditorView("note-ideas")
	goldenOpen(p, view)
	p.Push(p.conflictView(edit))

	goldenChoose(t, p, 2, "Compare")
	if top := topView(p); top.Title != "Compare unsaved changes" {
		t.Fatalf("top view = %q, want the diff", top.Title)
	}
	p.Pop()
	if top := topView(p); top.Title != "Unsaved changes found" {
		t.Fatalf("after Escape the top view is %q, want the choices", top.Title)
	}
}

// Snippets are found by their keyword at the root, and a keyword is one row whose action pastes the snippet.
func TestRootFindsASnippetByItsKeyword(t *testing.T) {
	p := goldenPalette(t)
	goldenQuery(p, ";sig")
	row := findRow(t, p, "Email signature")
	if row.Kind != search.KindSnippets {
		t.Fatalf("kind = %q, want Snippets", row.Kind)
	}
	if row.LearnKey != "snippet:snip-sig" {
		t.Fatalf("learn key = %q", row.LearnKey)
	}
}

// Paste refused by the platform leaves the snippet on the clipboard for a manual paste, and nothing is lost.
func TestPasteRefusedLeavesTheSnippetOnTheClipboard(t *testing.T) {
	p := goldenPalette(t)
	paster := p.ports.Paster.(*fake.Paster)
	paster.FailNext(fake.OpPasteIntoFrontmost, platform.ErrUnsupported)
	clip := p.ports.Clipboard.(*fake.Clipboard)
	snip, ok := p.snips.Get("snip-sig")
	if !ok {
		t.Fatal("fixture snippet missing")
	}

	p.clip = clipEngineOver(p.ports)
	p.pasteSnippet(&recordingHost{}, snip)

	deadline := time.Now().Add(watchdog)
	for {
		got, _ := clip.ReadText()
		if got == snip.Text {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("clipboard = %q, want the snippet text", got)
		}
		time.Sleep(time.Millisecond)
	}
}

// The snippets list paste action is the first action when the platform can paste, and copy when it cannot.
func TestSnippetPrimaryActionFollowsThePasteCapability(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.snippetsView())
	snip, _ := p.snips.Get("snip-sig")
	if got := p.snippetItem(snip).Actions[0].Title; got != "Paste" {
		t.Fatalf("primary action = %q, want Paste", got)
	}
	var withoutPaste []platform.Capability
	for _, c := range platform.AllCapabilities() {
		if c != platform.CapabilityClipboardPaste {
			withoutPaste = append(withoutPaste, c)
		}
	}
	p.ports.Capabilities = platform.NewCapabilities(withoutPaste...)
	if got := p.snippetItem(snip).Actions[0].Title; got != "Copy to Clipboard" {
		t.Fatalf("primary action without paste = %q, want Copy to Clipboard", got)
	}
}
