package notes

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// windowID names the floating window: one per extension, restored at launch under the same key.
const windowID = "floating-notes"

// windowTitle titles the floating window the toggle and the restore show, as 1.x titles it. A window opened on a new
// note takes the note's title instead.
const windowTitle = "Floating Notes"

// Floating window metrics, as the 1.x window has them.
const (
	windowWidth  = 640
	windowHeight = 460
)

// floatingWindowOptions are the options of the floating window: always on top, with a hidden title bar, a remembered
// frame, a persistent restore key, and the title given.
func floatingWindowOptions(title string) api.WindowOptions {
	alwaysOnTop := true
	return api.WindowOptions{
		ID: windowID, RestoreKey: windowID, Title: title, TitleBar: "hidden",
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
	options := floatingWindowOptions(windowTitle)
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
	create := ctx.Windows().Create(editorView(ctx, note, modeWindow), floatingWindowOptions(noteTitle(note)))
	return api.Result{Next: &create}, nil
}
