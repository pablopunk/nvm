package app

import (
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	notesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/notes"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// floatingGoldens is the group of the floating window's goldens under testdata/golden/floating, at the window's
// default size. The window content is drawn as the window draws it, without the window's own chrome.
var floatingGoldens = golden.Options{Group: "floating", Width: 640, Height: 460}

// floatingTallGoldens is the window at a height that shows a whole note above the actions: a note this long scrolls
// to its caret in the default height, which hides its first lines.
var floatingTallGoldens = golden.Options{Group: "floating", Width: 640, Height: 640}

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

// builtInWindowPalette draws a built-in's view the way the floating window draws it: a companion palette, shown on a
// window the golden never displays, so the golden is the window's content.
func builtInWindowPalette(t *testing.T, m *manager.Manager, view api.View) *palette.Palette {
	t.Helper()
	page := extview.NewSession(syncViewDeps(m), notesbuiltin.ID, view).Palette()
	p := palette.New(palette.Options{Root: page, Clock: fake.NewClock(), Synchronous: true, Layout: palette.LayoutCompanion})
	p.Attach(&testWindow{})
	p.Show()
	return p
}

// goldenNoteWithText creates a note and saves markdown into it, as the editor does, so the note has its saved version.
func goldenNoteWithText(t *testing.T, p *App, markdown string) notes.Note {
	t.Helper()
	n, err := p.notes.Create(goldenNow)
	if err != nil {
		t.Fatal(err)
	}
	saved, _, err := p.notes.SaveIfUnchanged(n.ID, markdown, n.UpdatedAt, goldenNow.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

// The floating window with a note that has headings, bold, a list and code: the editor draws the Markdown styled.
func TestFloatingNoteWindowShowsMarkdownStyled(t *testing.T) {
	p := goldenPalette(t)
	m := goldenNotesManager(t, p)
	n := goldenNoteWithText(t, p, floatingGoldenMarkdown)
	inv := api.Invocation{Ref: api.ActionRef{ID: notesbuiltin.OpenNoteActionID, Args: goldenNoteArgs(n.ID, notesbuiltin.ModeWindow)}}
	window := builtInWindowPalette(t, m, builtInInvocationView(t, m, notesbuiltin.ID, inv))
	golden.AssertMatrix(t, "note-markdown", window.View, floatingTallGoldens)
}

// The conflict replaces the editor with its two choices, as 1.x's window does.
func TestFloatingNoteWindowConflictChoices(t *testing.T) {
	p := goldenPalette(t)
	m := goldenNotesManager(t, p)
	window := builtInWindowPalette(t, m, builtInInvocationView(t, m, notesbuiltin.ID, goldenConflictInvocation(notesbuiltin.ModeWindow)))
	golden.AssertMatrix(t, "note-conflict", window.View, floatingGoldens)
}

// All notes in the window: the search, the count, New note and the notes newest first.
func TestFloatingNoteWindowAllNotes(t *testing.T) {
	p := goldenPalette(t)
	m := goldenNotesManager(t, p)
	inv := api.Invocation{Ref: api.ActionRef{ID: notesbuiltin.AllNotesActionID, Args: map[string]string{notesbuiltin.ArgMode: notesbuiltin.ModeWindow}}}
	window := builtInWindowPalette(t, m, builtInInvocationView(t, m, notesbuiltin.ID, inv))
	golden.AssertMatrix(t, "all-notes", window.View, floatingGoldens)
}

// The ⌘K menu over the note: its actions with their key caps, the first row selected, and the footer's Back hint.
func TestFloatingNoteWindowActionsMenu(t *testing.T) {
	p := goldenPalette(t)
	m := goldenNotesManager(t, p)
	n := goldenNoteWithText(t, p, floatingGoldenMarkdown)
	inv := api.Invocation{Ref: api.ActionRef{ID: notesbuiltin.OpenNoteActionID, Args: goldenNoteArgs(n.ID, notesbuiltin.ModeWindow)}}
	window := builtInWindowPalette(t, m, builtInInvocationView(t, m, notesbuiltin.ID, inv))
	window.OpenActionMenu(0)
	golden.AssertMatrix(t, "actions-menu", window.View, floatingTallGoldens)
}
