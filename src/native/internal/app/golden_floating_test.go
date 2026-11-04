package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// floatingGoldens is the group of the floating window's goldens under testdata/golden/floating, at the window's
// default size. The window content is drawn as the window draws it, without the window's own chrome.
var floatingGoldens = golden.Options{Group: "floating", Width: 640, Height: 460}

// floatingGoldenMarkdown is a note with a heading, bold and italic words, inline code, a list, a quotation and a
// block of code: the styles the rich text editor shows as they are typed.
var floatingGoldenMarkdown = strings.Join([]string{
	"# Release plan",
	"",
	"Ship the **floating** window with *italic* words and `inline code`.",
	"",
	"## Checklist",
	"",
	"- keyboard first",
	"- ship the goldens",
	"",
	"> Keep it local.",
	"",
	"```",
	"go test ./...",
	"```",
}, "\n")

// The floating window with a note that has headings, bold, a list and code: the editor draws the Markdown styled.
func TestFloatingNoteWindowShowsMarkdownStyled(t *testing.T) {
	p := goldenPalette(t)
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	edit, _ := p.acquireEdit(n.ID)
	if err := edit.sess.Edit(floatingGoldenMarkdown); err != nil {
		t.Fatal(err)
	}
	golden.AssertMatrix(t, "note-markdown", func(c *ui.Context) { floatingNoteWindow(c, p, edit) }, floatingGoldens)
}

// The conflict replaces the editor with its two choices, as 1.x's window does.
func TestFloatingNoteWindowConflictChoices(t *testing.T) {
	p := goldenPaletteWith(t, true)
	edit, _ := p.acquireEdit("note-ideas")
	golden.AssertMatrix(t, "note-conflict", func(c *ui.Context) { floatingNoteWindow(c, p, edit) }, floatingGoldens)
}

// All notes in the window: the search, the count, New note and the notes newest first.
func TestFloatingNoteWindowAllNotes(t *testing.T) {
	p := goldenPalette(t)
	edit, _ := p.acquireEdit("note-ideas")
	p.floatPane = floatingPaneState{pane: paneAllNotes}
	golden.AssertMatrix(t, "all-notes", func(c *ui.Context) { floatingNoteWindow(c, p, edit) }, floatingGoldens)
}

// The ⌘K menu over the note: its actions with their key caps, the first row selected, and the footer's Back hint.
func TestFloatingNoteWindowActionsMenu(t *testing.T) {
	p := goldenPalette(t)
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	edit, _ := p.acquireEdit(n.ID)
	if err := edit.sess.Edit(floatingGoldenMarkdown); err != nil {
		t.Fatal(err)
	}
	p.floatPane.actionsOpen = true
	golden.AssertMatrix(t, "actions-menu", func(c *ui.Context) { floatingNoteWindow(c, p, edit) }, floatingGoldens)
}
