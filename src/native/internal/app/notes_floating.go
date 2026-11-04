package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Floating notes: a note in its own always-on-top window. The window's frame is remembered per note and the
// windows that were open are opened again at launch, on the displays that are connected then.

// openFloatingNote shows a note in a floating window, or brings its window forward when it is open already.
func (p *App) openFloatingNote(id string) {
	if p.notes == nil {
		return
	}
	n, ok := p.notes.Get(id)
	if !ok {
		p.Toast("This note no longer exists")
		return
	}
	edit, ok := p.acquireEdit(id)
	if !ok {
		return
	}
	if edit.window != nil {
		edit.window.Focus()
		return
	}
	frame := p.floatingFrame(n)
	w := mygo.NewWindow(mygo.WindowOptions{
		Title: n.Title(), Width: frame.Width, Height: frame.Height, X: frame.X, Y: frame.Y,
		AlwaysOnTop: true, TitleBarStyle: mygo.TitleBarHidden,
		Content: ui.View(func(c *ui.Context) { floatingNoteWindow(c, edit) }),
	})
	edit.window = w
	w.OnMove(func() { p.rememberFloatingFrame(edit.id, w) })
	w.OnResize(func() { p.rememberFloatingFrame(edit.id, w) })
	w.OnClosed(func() { p.floatingNoteClosed(edit) })
	_ = p.notes.SetFloating(id, true)
	w.Show()
}

// newFloatingNote is the root command: it creates a note and opens it in a floating window.
func (p *App) newFloatingNote() {
	if p.notes == nil {
		p.Toast("Quick Notes could not be opened")
		return
	}
	n, err := p.notes.Create(clock())
	if err != nil {
		p.Toast(noteErrorMessage(err))
		return
	}
	p.openFloatingNote(n.ID)
}

// restoreFloatingNotes opens the windows that were open when the app last ran.
func (p *App) restoreFloatingNotes() {
	if p.notes == nil {
		return
	}
	for _, n := range p.notes.Floating() {
		p.openFloatingNote(n.ID)
	}
}

// floatingFrame is where a note's window goes: its remembered frame moved onto a connected display, or a default
// place on the display under the pointer.
func (p *App) floatingFrame(n notes.Note) notes.Frame {
	if n.Frame != nil {
		return notes.FitFrame(*n.Frame, p.displays())
	}
	return notes.DefaultFrame(p.cursorWorkArea())
}

// displays lists the connected displays, or none when the screens port is absent.
func (p *App) displays() []platform.Display {
	if p.ports.Screens == nil {
		return nil
	}
	return p.ports.Screens.Displays()
}

// cursorWorkArea is the work area of the display under the pointer, or a fixed area when the screens port is
// absent.
func (p *App) cursorWorkArea() platform.Rect {
	if p.ports.Screens == nil {
		return platform.Rect{Width: 1280, Height: 800}
	}
	return p.ports.Screens.NearestDisplay(p.ports.Screens.CursorPoint()).WorkArea
}

// rememberFloatingFrame stores where a floating window is now. Moves and resizes are frequent; the store
// debounces the write.
func (p *App) rememberFloatingFrame(id string, w *mygo.Window) {
	x, y := w.Position()
	width, height := w.Size()
	_ = p.notes.SetFrame(id, notes.Frame{X: x, Y: y, Width: width, Height: height})
}

// floatingNoteClosed runs when a floating window closes. Closing it keeps the note and its frame, and stops it
// from opening at the next launch; quitting the app keeps it floating so the next launch restores it.
func (p *App) floatingNoteClosed(edit *noteEdit) {
	edit.window = nil
	if !p.quitting {
		_ = p.notes.SetFloating(edit.id, false)
	}
	p.releaseEdit(edit)
}

// floatingNoteWindow is the content of a floating window, themed with Nevermind's tokens.
func floatingNoteWindow(c *ui.Context, edit *noteEdit) {
	ui.Themed(c, design.WindowTheme(c.Theme().Dark)).Fill().Children(func() {
		floatingNoteBody(c, edit)
	})
}

// floatingNoteBody is the content of a floating window: the note's text, with the same autosave as the editor.
func floatingNoteBody(c *ui.Context, edit *noteEdit) {
	syncBuffer(edit)
	if _, pending := edit.sess.Conflict(); pending {
		conflictNotice(c, "Unsaved changes from another session are waiting. Open this note in Quick Notes to choose which version to keep.")
		return
	}
	ta := ui.TextArea(c, &edit.text).Fill().Padding(12).FontSize(14).Placeholder(editorPlaceholder).AutoFocus()
	if ta.Changed() {
		_ = edit.sess.Edit(edit.text)
	}
}
