package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// floatingEditorTester renders a note's floating body in a window-sized frame and focuses its editor, as the window
// does on its first frame.
func floatingEditorTester(t *testing.T, p *App, id string) (*ui.Tester, *noteEdit) {
	t.Helper()
	edit, ok := p.acquireEdit(id)
	if !ok {
		t.Fatalf("note %q does not open", id)
	}
	tt := ui.NewTester(func(c *ui.Context) { floatingNoteBody(c, edit) }, 360, 300)
	tt.Frame()
	return tt, edit
}

// typeKeys types s one character at a time, as a keyboard does: the Markdown shortcuts run per typed character.
func typeKeys(tt *ui.Tester, s string) {
	for _, r := range s {
		tt.Type(string(r))
	}
}

// Typing "# " at the start of a line makes a heading, and the saved Markdown keeps the marker, as 1.x saves it.
func TestFloatingNoteHeadingShortcutStylesTheLineAndSavesMarkdown(t *testing.T) {
	p := goldenPalette(t)
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	tt, edit := floatingEditorTester(t, p, n.ID)

	typeKeys(tt, "# Hello")
	tt.Frame()

	if got := edit.doc.Text().PlainText(); got != "Hello" {
		t.Fatalf("plain text = %q, want the heading text without its marker", got)
	}
	if level := edit.doc.Text().ParagraphStyleAt(0).Heading; level != 1 {
		t.Fatalf("heading level = %d, want 1", level)
	}
	if edit.text != "# Hello" || edit.sess.Text() != "# Hello" {
		t.Fatalf("saved text = %q / %q, want the Markdown", edit.text, edit.sess.Text())
	}
}

// Typing "**" around words makes them bold, and the saved text keeps the asterisks.
func TestFloatingNoteBoldShortcutStylesTheWordAndSavesMarkdown(t *testing.T) {
	p := goldenPalette(t)
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	tt, edit := floatingEditorTester(t, p, n.ID)

	typeKeys(tt, "some **bold** text")
	tt.Frame()

	if got := edit.doc.Text().PlainText(); got != "some bold text" {
		t.Fatalf("plain text = %q", got)
	}
	if edit.sess.Text() != "some **bold** text" {
		t.Fatalf("saved text = %q, want the Markdown", edit.sess.Text())
	}
}

// A note saved as Markdown opens styled: its marker characters are not shown as text.
func TestFloatingNoteOpensSavedMarkdownAsStyledText(t *testing.T) {
	p := goldenPalette(t)
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	edit, _ := p.acquireEdit(n.ID)
	if err := edit.sess.Edit("### hello there\n\n**asdf**"); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) { floatingNoteBody(c, edit) }, 360, 300)
	tt.Frame()

	text := edit.doc.Text()
	if got := text.PlainText(); got != "hello there\nasdf" {
		t.Fatalf("plain text = %q, want the words without markers", got)
	}
	if level := text.ParagraphStyleAt(0).Heading; level != 3 {
		t.Fatalf("heading level = %d, want 3", level)
	}
}
