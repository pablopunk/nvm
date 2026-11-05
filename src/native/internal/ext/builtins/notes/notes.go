// Package notes is the built-in that keeps the user's Markdown notes. The Floating Notes command lists them in the
// palette, one floating window shows one note at a time, and Toggle, New and the window's ⌘N and ⌘O open them. The
// notes live in the host's notes store and are read and written through Context().Notes(); the built-in keeps no
// state of its own, and a window restored at launch is rebuilt from the store.
//
// Layer: ext. The package imports api, search and the standard library.
package notes

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// ID is the built-in's identity. Its command IDs match the 1.x extension's, so shortcuts bound in 1.x apply.
const ID = "nevermind.floating-notes"

// Command IDs. ListCommandID is the palette list, ToggleCommandID shows or hides the floating window, and
// NewWindowCommandID and NewNoteCommandID make a note in the window and in the palette.
const (
	ListCommandID      = "floating-notes"
	ToggleCommandID    = "toggle-floating-notes"
	NewWindowCommandID = "new-floating-note"
	NewNoteCommandID   = "new-note"
)

// Titles and the search placeholder the palette list shows, as 1.x shows them.
const (
	listTitle         = "Floating Notes"
	listPlaceholder   = "Find a note"
	newNoteTitle      = "New note"
	newNoteSubtitle   = "Start a fresh Markdown note"
	openInWindowTitle = "Open in Window"
	deleteTitle       = "Delete note"
)

// Refs of the actions the rows, the editor, the conflict view and the window run. The args carry the note's id, the
// mode the view is shown in, and where a view goes: a push from the list, or a replace of the view above.
const (
	refNewNote        = "new-note-action"
	refOpenNote       = "open-note"
	refOpenInWindow   = "open-note-in-window"
	refDeleteFromList = "delete-note-from-list"
	refDeleteShown    = "delete-shown-note"
	refAllNotes       = "all-notes"
	refAutosave       = "autosave-note"
	refConflict       = "note-conflict"
	argID             = "id"
	argMode           = "mode"
	argNav            = "nav"
	argText           = "text"
	modePalette       = "palette"
	modeWindow        = "window"
	navPush           = "push"
	navReplace        = "replace"
)

// Extension is the Floating Notes built-in.
type Extension struct{}

// New returns the Floating Notes built-in.
func New() Extension {
	return Extension{}
}

// Manifest identifies the built-in. It needs the notes capability, which is the store's service.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{
		APIVersion:   "1.0",
		Identity:     api.Identity{ID: ID, Title: listTitle},
		Capabilities: []api.Capability{api.CapabilityNotes},
	}
}

// Commands lists the palette list, which the shortcut Command+Alt+N opens, and three commands searched by name only.
// Toggle and the make-a-note commands keep the 1.x shortcuts, so the two windows commands can be bound.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	yellow := &api.Appearance{Foreground: api.ColorYellow}
	return []api.Command{
		{
			ID: ListCommandID, Title: listTitle, Subtitle: "Jot something down", Icon: "sticky-note", Appearance: yellow,
			Keywords: []string{"notes", "note", "scratch", "memo", "floating"}, Score: 12,
			Section: search.KindCommands, LearnKey: "cmd:" + listTitle, Customizable: true,
		},
		{
			ID: ToggleCommandID, Title: "Toggle Floating Notes", Subtitle: "Show or hide your floating Markdown notes",
			Icon: "sticky-note", Appearance: yellow, Placement: []api.Placement{api.PlacementSearch},
			Keywords:        []string{"notes", "note", "float", "floating window", "sticky", "toggle"},
			DismissAfterRun: true,
			Section:         search.KindCommands, LearnKey: "cmd:Toggle Floating Notes",
		},
		{
			ID: NewWindowCommandID, Title: "New Floating Note", Subtitle: "Start a fresh local Markdown note",
			Icon: "sticky-note", Appearance: yellow, Placement: []api.Placement{api.PlacementSearch}, Customizable: true,
			Keywords:        []string{"note", "float", "floating window", "sticky"},
			DismissAfterRun: true,
			Section:         search.KindCommands, LearnKey: "cmd:New Floating Note",
		},
		{
			ID: NewNoteCommandID, Title: "New Note", Icon: "plus", Placement: []api.Placement{api.PlacementSearch},
			Keywords: []string{"note", "create", "scratch"},
			Section:  search.KindCommands, LearnKey: "cmd:New Note",
		},
	}, nil
}

// RunAction runs the commands, the rows' and the editor's actions, the autosave, the draft conflict and the conflict
// view's choices.
func (e Extension) RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	args := in.Ref.Args
	switch in.Ref.ID {
	case ListCommandID:
		return showList(ctx, modePalette, "")
	case ToggleCommandID:
		return toggleWindow(ctx)
	case NewWindowCommandID:
		return newWindow(ctx)
	case NewNoteCommandID, refNewNote:
		return newNote(ctx, args)
	case refOpenNote:
		return openNote(ctx, args)
	case refOpenInWindow:
		return openInWindow(ctx, args[argID])
	case refAllNotes:
		return showList(ctx, args[argMode], args[argNav])
	case refDeleteFromList:
		return deleteFromList(ctx, args)
	case refDeleteShown:
		return deleteShown(ctx, args)
	case refAutosave:
		_, err := saveLatest(ctx, args[argID], in.EditorContent)
		return api.Result{}, err
	case refConflict:
		return conflictView(ctx, args, in.DraftConflict)
	case api.NoteConflictRestoreRef:
		return restoreUnsaved(ctx, args)
	case api.NoteConflictKeepRef:
		return keepSaved(ctx, args)
	}
	return api.Result{}, fmt.Errorf("the floating notes built-in has no action %q: %w", in.Ref.ID, api.ErrInvalidInput)
}

// RestoreWindow rebuilds the floating window after relaunch, on the most recent note. Any other key is not this
// built-in's window.
func (Extension) RestoreWindow(ctx api.Context, restoreKey string) (*api.WindowRestore, error) {
	if restoreKey != windowID {
		return nil, nil
	}
	note, err := ctx.Notes().Recent()
	if err != nil {
		return nil, err
	}
	options := floatingWindowOptions(note)
	return &api.WindowRestore{View: editorView(ctx, note, modeWindow), Options: &options}, nil
}
