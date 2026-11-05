package notes

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// windowID names the floating window: one per extension, restored at launch under the same key.
const windowID = "floating-notes"

// Floating window metrics, as the 1.x window has them.
const (
	windowWidth  = 640
	windowHeight = 460
)

// floatingWindowOptions are the options of the floating window: always on top, with a hidden title bar, a remembered
// frame, and a persistent restore key. note titles a window that opens on a new note.
func floatingWindowOptions(note api.Note) api.WindowOptions {
	alwaysOnTop := true
	return api.WindowOptions{
		ID: windowID, RestoreKey: windowID, Title: noteTitle(note), TitleBar: "hidden",
		Width: windowWidth, Height: windowHeight, AlwaysOnTop: &alwaysOnTop,
		Persistent: true, RemembersFrame: true,
	}
}

// toggleWindow is the Toggle Floating Notes command: it hides the window when it is open, and otherwise shows the most
// recent note in it, creating a note when there is none.
func toggleWindow(ctx api.Context) (api.Result, error) {
	note, err := ctx.Notes().Recent()
	if err != nil {
		return api.Result{}, err
	}
	view := editorView(ctx, note, modeWindow)
	options := floatingWindowOptions(note)
	toggle := ctx.Windows().Toggle(api.WindowRef{ID: windowID, View: &view, Options: &options})
	return api.Result{Next: &toggle}, nil
}

// newWindow is the New Floating Note command: it creates a note and shows it in the floating window.
func newWindow(ctx api.Context) (api.Result, error) {
	note, err := ctx.Notes().Create()
	if err != nil {
		return api.Result{}, err
	}
	return createWindow(ctx, note)
}

// openInWindow shows a note in the floating window, as Open in Window does from the list.
func openInWindow(ctx api.Context, id string) (api.Result, error) {
	note, err := savedNote(ctx, id)
	if err != nil {
		return api.Result{}, err
	}
	return createWindow(ctx, note)
}

// createWindow opens the floating window on note. The window opens on the note's editor, titled with the note.
func createWindow(ctx api.Context, note api.Note) (api.Result, error) {
	create := ctx.Windows().Create(editorView(ctx, note, modeWindow), floatingWindowOptions(note))
	return api.Result{Next: &create}, nil
}
