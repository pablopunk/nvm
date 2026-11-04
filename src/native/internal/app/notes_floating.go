package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// floatingWindowTitle is the window's title when a command opens it, as 1.x titles it.
const floatingWindowTitle = "Floating Notes"

// Floating window metrics of the 1.x window: the minimum size, and the strip under the window controls that the
// content leaves for them.
const (
	floatingMinWidth    = 320
	floatingMinHeight   = 240
	floatingControlsGap = 36
)

// floatingSurface is the part of the floating window the app drives: it places, focuses and closes the window. The
// MyGo window implements it; tests give the app a fake.
type floatingSurface interface {
	Focus()
	Close()
	Position() (x, y int)
	Size() (width, height int)
}

// floatingWindow is the one floating note window, as 1.x has one: it shows one note at a time, and opening another
// note in it replaces the note. edit is the note it shows.
type floatingWindow struct {
	window floatingSurface
	edit   *noteEdit
}

// openFloatingNote shows a note in the floating window. When the window is open already, it shows the note in it
// instead, and brings the window forward. title is the window's title when it opens.
func (p *App) openFloatingNote(id, title string) {
	if p.notes == nil {
		return
	}
	edit, ok := p.acquireEdit(id)
	if !ok {
		p.Toast("This note no longer exists")
		return
	}
	if p.floating != nil {
		p.showInFloatingWindow(edit)
		p.floating.window.Focus()
		return
	}
	frame := p.floatingFrame()
	w := mygo.NewWindow(mygo.WindowOptions{
		Title: title, Width: frame.Width, Height: frame.Height, X: frame.X, Y: frame.Y,
		AlwaysOnTop: true, TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 12, Y: 12},
		MinWidth: floatingMinWidth, MinHeight: floatingMinHeight,
		Content: ui.View(func(c *ui.Context) {
			if p.floating != nil {
				floatingNoteWindow(c, p, p.floating.edit)
			}
		}),
	})
	p.floating = &floatingWindow{window: w, edit: edit}
	w.OnMove(p.rememberFloatingFrame)
	w.OnResize(p.rememberFloatingFrame)
	w.OnClosed(p.floatingWindowClosed)
	_ = p.notes.SetFloating(id, true)
	w.Show()
}

// showInFloatingWindow puts another note in the open floating window. The window keeps its place; the note it
// showed ends its session when nothing else shows it.
func (p *App) showInFloatingWindow(edit *noteEdit) {
	previous := p.floating.edit
	if previous == edit {
		return
	}
	p.floating.edit = edit
	_ = p.notes.SetFloating(previous.id, false)
	_ = p.notes.SetFloating(edit.id, true)
	p.releaseEdit(previous)
}

// toggleFloatingNotes is the Toggle Floating Notes command: it closes the floating window when it is open, and
// otherwise shows the most recent note in it, creating one when there is none.
func (p *App) toggleFloatingNotes() {
	if p.floating != nil {
		p.floating.window.Close()
		return
	}
	if p.notes == nil {
		p.Toast("Quick Notes could not be opened")
		return
	}
	id, err := p.mostRecentNoteID()
	if err != nil {
		p.Toast(noteErrorMessage(err))
		return
	}
	p.openFloatingNote(id, floatingWindowTitle)
}

// mostRecentNoteID is the note saved last, or a new note when there is none yet.
func (p *App) mostRecentNoteID() (string, error) {
	if all := p.notes.List(); len(all) > 0 {
		return all[0].ID, nil
	}
	n, err := p.notes.Create(clock())
	return n.ID, err
}

// newFloatingNote is the root command: it creates a note and shows it in the floating window.
func (p *App) newFloatingNote() {
	if p.notes == nil {
		p.Toast("Floating Notes could not be opened")
		return
	}
	n, err := p.notes.Create(clock())
	if err != nil {
		p.Toast(noteErrorMessage(err))
		return
	}
	p.openFloatingNote(n.ID, n.Title())
}

// showNextFloatingNote puts the most recent note in the floating window after the note it showed was deleted, as
// 1.x does, creating a note when none is left.
func (p *App) showNextFloatingNote() {
	id, err := p.mostRecentNoteID()
	if err != nil {
		p.Toast(noteErrorMessage(err))
		return
	}
	edit, ok := p.acquireEdit(id)
	if !ok {
		return
	}
	p.floating.edit = edit
	_ = p.notes.SetFloating(id, true)
}

// restoreFloatingNotes reopens the floating window when it was open at the last quit, with the most recent note,
// as 1.x does.
func (p *App) restoreFloatingNotes() {
	if p.notes == nil || len(p.notes.Floating()) == 0 {
		return
	}
	id, err := p.mostRecentNoteID()
	if err != nil {
		return
	}
	p.openFloatingNote(id, floatingWindowTitle)
}

// floatingFrame is where the floating window goes: the frame it was last at, moved onto a connected display, or a
// default place on the display under the pointer.
func (p *App) floatingFrame() notes.Frame {
	if frame, ok := p.notes.FloatingFrame(); ok {
		return notes.FitFrame(frame, p.displays())
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

// rememberFloatingFrame stores where the floating window is now. Moves and resizes are frequent; the store
// debounces the write.
func (p *App) rememberFloatingFrame() {
	x, y := p.floating.window.Position()
	width, height := p.floating.window.Size()
	_ = p.notes.SetFloatingFrame(notes.Frame{X: x, Y: y, Width: width, Height: height})
}

// floatingWindowClosed runs when the floating window closes. Closing it keeps the note and its frame, and stops the
// window from reopening at the next launch; quitting the app keeps it open so the next launch restores it.
func (p *App) floatingWindowClosed() {
	edit := p.floating.edit
	p.floating = nil
	p.floatPane = floatingPaneState{}
	if !p.quitting {
		_ = p.notes.SetFloating(edit.id, false)
	}
	p.releaseEdit(edit)
}

// isFloatingEdit reports whether the floating window shows the note of edit.
func (p *App) isFloatingEdit(edit *noteEdit) bool {
	return p.floating != nil && p.floating.edit == edit
}
