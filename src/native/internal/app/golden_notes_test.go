package app

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	notesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/notes"
	snippetsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// The goldens of Floating Notes and Snippets. The notes views are the built-in's own, opened over the goldens' notes
// store; the snippets list is the Snippets built-in's.

// goldenUnsavedIdeas is the text the editor kept unsaved for note-ideas when the conflict golden opens.
const goldenUnsavedIdeas = "Ideas\nPalette chips for filters\n- keyboard first\n- [ ] ship the goldens\n- sketched on the train"

// goldenNotesManager serves the built-ins over the goldens' notes store, on the goldens' clock, so the times the views
// show are fixed.
func goldenNotesManager(t *testing.T, p *App) *manager.Manager {
	t.Helper()
	builtins := builtInExtensions(builtInInputs{Zone: time.UTC, Snippets: goldenLegacySnippets(t)})
	clock := fake.NewClock()
	clock.Advance(goldenNow.Sub(clock.Now()))
	return newManagerWithApps(t, builtins, clock, nil, nil, extensionPorts{notes: func() *notes.Store { return p.notes }})
}

// goldenNoteArgs are the args that name a note and the mode its view is shown in.
func goldenNoteArgs(id, mode string) map[string]string {
	return map[string]string{notesbuiltin.ArgID: id, notesbuiltin.ArgMode: mode}
}

// goldenNoteEditorInvocation opens note-ideas in mode, as the notes list opens a row.
func goldenNoteEditorInvocation(mode string) api.Invocation {
	return api.Invocation{Ref: api.ActionRef{ID: notesbuiltin.OpenNoteActionID, Args: goldenNoteArgs("note-ideas", mode)}}
}

// goldenConflictInvocation opens the conflict choices over note-ideas in mode, with the text the editor kept unsaved.
func goldenConflictInvocation(mode string) api.Invocation {
	return api.Invocation{
		Ref:           api.ActionRef{ID: notesbuiltin.ConflictActionID, Args: goldenNoteArgs("note-ideas", mode)},
		DraftConflict: &api.DraftConflict{StoredContent: goldenUnsavedIdeas},
	}
}

// goldenNoteEditorPage is the editor of note-ideas in the palette.
func goldenNoteEditorPage(t *testing.T, p *App) *palette.View {
	t.Helper()
	return builtInInvocationPage(t, goldenNotesManager(t, p), notesbuiltin.ID, goldenNoteEditorInvocation(notesbuiltin.ModePalette))
}

func TestPaletteNotesList(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, builtInViewPage(t, goldenNotesManager(t, p), notesbuiltin.ID, api.ActionRef{ID: notesbuiltin.ListCommandID}))
	p.Choose(1)
	golden.AssertMatrix(t, "notes-list", p.View, paletteGoldens)
}

func TestPaletteNoteEditor(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenNoteEditorPage(t, p))
	golden.AssertMatrix(t, "notes-editor", p.View, paletteGoldens)
}

// The conflict is the editor under the choices: the choices are on top, as they open when a note with unsaved
// changes is opened.
func TestPaletteNoteConflictChoices(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, goldenNoteEditorPage(t, p))
	p.Push(builtInInvocationPage(t, goldenNotesManager(t, p), notesbuiltin.ID, goldenConflictInvocation(notesbuiltin.ModePalette)))
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
