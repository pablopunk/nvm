package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.editor is the dev fixture of the editor view and its drafts. The note editor autosaves under note-1 at version
// v1; Save commits the draft at v2, and a conflict is resolved by discarding the stored draft. The note's saved text is
// fixed, so a relaunch can show a draft that survived a crash.

const noteKey = "note-1"

const noteSavedText = "A saved note that the editor opens with."

// SearchItems offers the fixture's view as a root row for queries that name it, so the palette can open it.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if !strings.Contains(strings.ToLower("Editor fixture: open the note"), strings.ToLower(query)) {
		return nil, nil
	}
	return []api.Item{{ID: "note", Title: "Editor fixture: open the note", Primary: api.Primary(api.Run("note", "Open", nil))}}, nil
}

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "note", Title: "Note fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "note":
		view := api.EditorView("Note", noteSavedText, api.Run("commit", "Save", nil))
		view.Editor.Placeholder = "Write a note"
		view.Editor.Draft = &api.DraftOptions{Key: noteKey, Version: "v1", OnConflict: &api.ActionRef{ID: "resolve"}}
		return api.Result{View: &view}, nil
	case "commit":
		removed, err := ctx.Drafts().Commit(noteKey, "v2")
		if err != nil {
			return api.Result{}, err
		}
		title := fmt.Sprintf("saved: %s · draft removed: %v", in.EditorContent, removed)
		view := api.ListView("Saved", api.Item{ID: "saved", Title: title})
		return api.Result{View: &view}, nil
	case "resolve":
		if in.DraftConflict == nil {
			return api.Result{}, fmt.Errorf("no conflict was passed")
		}
		return api.Result{Draft: &api.DraftResolution{Key: in.DraftConflict.Key, Resolution: "reset"}}, nil
	}
	return api.Result{}, fmt.Errorf("unknown command %q", in.Ref.ID)
}
