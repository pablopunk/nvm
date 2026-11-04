package app

import (
	snippetsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/palette"
)

// The goldens of Floating Notes and Snippets: the list with its detail pane, the editor, the conflict choices, and the
// snippets list. The stores are the feature packages' own, opened over a fake disk seeded with the fixtures.

func TestPaletteNotesList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.notesView())
	p.Choose(1)
	golden.AssertMatrix(t, "notes-list", p.View, paletteGoldens)
}

func TestPaletteNoteEditor(t *testing.T) {
	p := goldenPalette(t)
	view, _ := p.noteEditorView("note-ideas")
	goldenOpen(p, view)
	golden.AssertMatrix(t, "notes-editor", p.View, paletteGoldens)
}

// The conflict is the editor under the choices: the choices are on top, as they open when a note with unsaved
// changes is opened.
func TestPaletteNoteConflictChoices(t *testing.T) {
	p := goldenPaletteWith(t, true)
	view, edit := p.noteEditorView("note-ideas")
	goldenOpen(p, view)
	p.Push(p.conflictView(edit))
	golden.AssertMatrix(t, "notes-conflict", p.View, paletteGoldens)
}

func TestPaletteSnippetsList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, snippetsBuiltInPage(t, goldenLegacySnippets(t), snippetsbuiltin.ListCommandID, nil))
	p.Choose(1)
	golden.AssertMatrix(t, "snippets-list", p.View, paletteGoldens)
}

// goldenChoose runs the action of the row at index, through the same path Enter takes.
func goldenChoose(t *testing.T, p *App, index int, action string) {
	t.Helper()
	results := p.Results()
	if index >= len(results) {
		t.Fatalf("row %d of %d", index, len(results))
	}
	for _, a := range results[index].Actions {
		if a.Title == action {
			a.Run(p.Palette)
			return
		}
	}
	t.Fatalf("row %d has no action %q", index, action)
}

// topView is the view on top of the palette's stack.
func topView(p *App) *palette.View {
	return p.Top()
}
