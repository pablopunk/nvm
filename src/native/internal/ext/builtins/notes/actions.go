package notes

import (
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// errNoteGone is the failure of a row or a window that names a note the store no longer holds.
var errNoteGone = errors.New("this note no longer exists")

// errNoteChanged is the failure of a write that met a newer saved version of the note.
var errNoteChanged = errors.New("this note changed elsewhere; reopen it to see the saved text")

// showList shows the notes in mode. A list opened from the palette root pushes; one opened from an editor, or in the
// window, replaces the view above it.
func showList(ctx api.Context, mode, nav string) (api.Result, error) {
	notes, err := ctx.Notes().List()
	if err != nil {
		return api.Result{}, err
	}
	view := listView(ctx, notes, mode)
	return show(mode, nav, ctx, &view), nil
}

// newNote makes a note and shows its editor in the mode and navigation the args name.
func newNote(ctx api.Context, args map[string]string) (api.Result, error) {
	note, err := ctx.Notes().Create()
	if err != nil {
		return api.Result{}, err
	}
	view := editorView(ctx, note, modeOf(args))
	return show(modeOf(args), args[argNav], ctx, &view), nil
}

// openNote shows a note's editor, in the mode and navigation the args name.
func openNote(ctx api.Context, args map[string]string) (api.Result, error) {
	note, err := savedNote(ctx, args[argID])
	if err != nil {
		return api.Result{}, err
	}
	view := editorView(ctx, note, modeOf(args))
	return show(modeOf(args), args[argNav], ctx, &view), nil
}

// deleteFromList removes a note the list chose, and shows the list without it. The store confirms with the user first.
func deleteFromList(ctx api.Context, args map[string]string) (api.Result, error) {
	if err := ctx.Notes().Delete(args[argID]); err != nil {
		return api.Result{}, err
	}
	notes, err := ctx.Notes().List()
	if err != nil {
		return api.Result{}, err
	}
	list := listView(ctx, notes, modeOf(args))
	return api.Result{
		Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: list.Items},
		Toast: &api.Toast{Message: "Note deleted", Tone: "success"},
	}, nil
}

// deleteShown removes the note the editor shows, and shows the most recent note in its place, creating one when none is
// left, as 1.x does.
func deleteShown(ctx api.Context, args map[string]string) (api.Result, error) {
	if err := ctx.Notes().Delete(args[argID]); err != nil {
		return api.Result{}, err
	}
	next, err := ctx.Notes().Recent()
	if err != nil {
		return api.Result{}, err
	}
	view := editorView(ctx, next, modeOf(args))
	result := show(modeOf(args), navReplace, ctx, &view)
	result.Toast = &api.Toast{Message: "Note deleted", Tone: "success"}
	return result, nil
}

// saveLatest writes markdown as the note's text. The write is against the note's current saved version, as 1.x's
// autosave is, and the draft is committed at the version it produced, so the editor does not conflict with its own save.
func saveLatest(ctx api.Context, id, markdown string) (api.Note, error) {
	current, err := savedNote(ctx, id)
	if err != nil {
		return api.Note{}, err
	}
	saved, changed, err := ctx.Notes().Update(id, markdown, current.UpdatedAt)
	if err != nil {
		return api.Note{}, err
	}
	if changed {
		return api.Note{}, errNoteChanged
	}
	_, err = ctx.Drafts().Commit(id, versionOf(saved))
	return saved, err
}

// conflictView answers a stored draft whose version differs from the saved note: the unsaved text, the saved text and
// the choices. The draft is not written until the user chooses.
func conflictView(ctx api.Context, args map[string]string, conflict *api.DraftConflict) (api.Result, error) {
	if conflict == nil {
		return api.Result{}, errors.New("the conflict handler needs a draft conflict")
	}
	saved, err := savedNote(ctx, args[argID])
	if err != nil {
		return api.Result{}, err
	}
	view := api.NoteConflictView(saved, conflict.StoredContent, compareMarkdown(saved.Markdown, conflict.StoredContent), args)
	return api.Result{View: &view, Navigation: api.NavReplace}, nil
}

// restoreUnsaved saves the unsaved text the conflict kept as the note's text, and shows the editor on it.
func restoreUnsaved(ctx api.Context, args map[string]string) (api.Result, error) {
	saved, err := saveLatest(ctx, args[argID], args[argText])
	if err != nil {
		return api.Result{}, err
	}
	return draftResolution(ctx, args, saved, "restore-old", &api.Toast{Message: "Unsaved changes restored", Tone: "success"}), nil
}

// keepSaved keeps the saved text and drops the unsaved one, then shows the editor on the saved note.
func keepSaved(ctx api.Context, args map[string]string) (api.Result, error) {
	saved, err := savedNote(ctx, args[argID])
	if err != nil {
		return api.Result{}, err
	}
	return draftResolution(ctx, args, saved, "reset", nil), nil
}

// draftResolution answers the draft conflict with the editor on note.
func draftResolution(ctx api.Context, args map[string]string, note api.Note, resolution string, toast *api.Toast) api.Result {
	view := editorView(ctx, note, modeOf(args))
	return api.Result{
		Draft: &api.DraftResolution{Key: note.ID, Resolution: resolution, View: &view, Navigation: api.NavReplace},
		Toast: toast,
	}
}

// savedNote returns the note with id, or says it no longer exists.
func savedNote(ctx api.Context, id string) (api.Note, error) {
	note, found, err := ctx.Notes().Get(id)
	if err != nil {
		return api.Note{}, err
	}
	if !found {
		return api.Note{}, errNoteGone
	}
	return note, nil
}

// show puts view on the palette stack or in the window. The window and any view opened from another editor replace the
// view above; the palette root's list pushes.
func show(mode, nav string, ctx api.Context, view *api.View) api.Result {
	if mode == modeWindow || nav == navReplace {
		return ctx.Navigation().Replace(view)
	}
	return ctx.Navigation().Push(view)
}

// modeOf is the mode a view is shown in: the window, or the palette when the args name no window.
func modeOf(args map[string]string) string {
	if args[argMode] == modeWindow {
		return modeWindow
	}
	return modePalette
}

// compareMarkdown is the Compare preview: the line diff of the saved text against the unsaved text, with removed lines
// marked "-" and added lines "+".
func compareMarkdown(saved, unsaved string) string {
	var b strings.Builder
	b.WriteString("````text\n")
	for _, line := range diffLines(saved, unsaved) {
		b.WriteString(diffMark(line.Op))
		b.WriteString(line.Text)
		b.WriteString("\n")
	}
	b.WriteString("````")
	return b.String()
}
