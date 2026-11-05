package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestPaletteNoteEditorTakesTypedText: a new note started from the Floating Notes list takes the keys typed right after
// its editor appears, as the rig's journey types them, and its text holds them.
func TestPaletteNoteEditorTakesTypedText(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.notesView())
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()
	itemTitled(t, p, "New Note").Actions[0].Run(p)
	awaitTopView(t, p, "Note")
	tt.Frame()
	typeKeys(tt, "this are selected text")
	tt.Frame()
	edit := p.edits[p.Top().Title]
	if edit == nil {
		edit = onlyEdit(t, p)
	}
	if !strings.Contains(edit.text, "this are selected text") {
		t.Fatalf("typed text did not reach the note: text = %q", edit.text)
	}
}

// onlyEdit returns the one note the palette is editing.
func onlyEdit(t *testing.T, p *App) *noteEdit {
	t.Helper()
	if len(p.edits) != 1 {
		t.Fatalf("the palette edits %d notes, want one", len(p.edits))
	}
	for _, edit := range p.edits {
		return edit
	}
	return nil
}
