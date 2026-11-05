package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A note opened from the list takes the keyboard in the palette: the first frame after it is pushed gives the Markdown
// editor the focus, so typing lands in the note and not in the hidden search field.
func TestOpenedNoteEditorTakesTypedText(t *testing.T) {
	p := goldenPalette(t)
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	view, edit := p.noteEditorView(n.ID)
	if view == nil {
		t.Fatal("the note does not open")
	}
	tt := ui.NewTester(p.View, 720, 470)
	tt.Frame()
	goldenOpen(p, view)
	tt.Frame()

	typeKeys(tt, "Rig note")
	tt.Frame()

	if got := edit.paletteRich.doc.Text().PlainText(); got != "Rig note" {
		t.Fatalf("editor text = %q, want the typed text; typing went elsewhere", got)
	}
}
