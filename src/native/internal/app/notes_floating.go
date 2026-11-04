package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Floating notes: a note in its own always-on-top window. The window's frame is remembered per note and the
// windows that were open are opened again at launch, on the displays that are connected then.

// floatingWindowTitle is the window's title when it opens from the command, as 1.x titles it.
const floatingWindowTitle = "Floating Notes"

// openFloatingNote shows a note in a floating window, or brings its window forward when it is open already. title is
// the window's title; the page's title bar is hidden, with the window controls over the page.
func (p *App) openFloatingNote(id, title string) {
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
		Title: title, Width: frame.Width, Height: frame.Height, X: frame.X, Y: frame.Y,
		AlwaysOnTop: true, TitleBarStyle: mygo.TitleBarHiddenInset, TrafficLightPosition: &mygo.Point{X: 12, Y: 12},
		MinWidth: floatingMinWidth, MinHeight: floatingMinHeight,
		Content: ui.View(func(c *ui.Context) { floatingNoteWindow(c, p, edit) }),
	})
	edit.window = w
	w.OnMove(func() { p.rememberFloatingFrame(edit.id, w) })
	w.OnResize(func() { p.rememberFloatingFrame(edit.id, w) })
	w.OnClosed(func() { p.floatingNoteClosed(edit) })
	_ = p.notes.SetFloating(id, true)
	w.Show()
}

// toggleFloatingNotes is the Toggle Floating Notes command: it closes the floating notes that are shown, and otherwise
// shows the most recent note floating, creating one when there is none.
func (p *App) toggleFloatingNotes() {
	if open := p.openFloatingEdits(); len(open) > 0 {
		for _, edit := range open {
			edit.window.Close()
		}
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

// openFloatingEdits lists the notes that have a floating window open.
func (p *App) openFloatingEdits() []*noteEdit {
	var open []*noteEdit
	for _, edit := range p.edits {
		if edit.window != nil {
			open = append(open, edit)
		}
	}
	return open
}

// mostRecentNoteID is the note saved last, or a new note when there is none yet.
func (p *App) mostRecentNoteID() (string, error) {
	if all := p.notes.List(); len(all) > 0 {
		return all[0].ID, nil
	}
	n, err := p.notes.Create(clock())
	return n.ID, err
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
	p.openFloatingNote(n.ID, n.Title())
}

// restoreFloatingNotes opens the windows that were open when the app last ran.
func (p *App) restoreFloatingNotes() {
	if p.notes == nil {
		return
	}
	for _, n := range p.notes.Floating() {
		p.openFloatingNote(n.ID, floatingWindowTitle)
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

// Floating window metrics of the 1.x window: the strip under the window controls, the minimum size, and the room
// around the header and the editor.
const (
	floatingMinWidth    = 320
	floatingMinHeight   = 240
	floatingControlsGap = 36
)

// floatingNoteWindow is the content of a floating window, laid out as the 1.x window is: a strip under the window
// controls that drags the window, then the note's header (its title and when it was edited), then the editor.
func floatingNoteWindow(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	ui.Themed(c, design.WindowTheme(c.Theme().Dark)).Fill().Background(k.Canvas).Children(func() {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Height(floatingControlsGap).Fill().DragWindow()
			ui.Column(c).Grow(1).Padding(0, 12, 12, 12).Gap(8).Children(func() {
				floatingNoteHeader(c, p, edit)
				floatingNoteBody(c, edit)
			})
		})
	})
}

// floatingNoteHeader is the note's title over its state: "Markdown · Edited 3m ago", as 1.x's editor header shows.
// The row drags the window, as the 1.x strip does.
func floatingNoteHeader(c *ui.Context, p *App, edit *noteEdit) {
	k := design.Tokens(c)
	title, subtitle := notes.Title(edit.text), "Markdown"
	if n, ok := p.notes.Get(edit.id); ok {
		subtitle += " · Edited " + notes.RelativeTime(clock(), n.UpdatedAt)
	}
	ui.Row(c).Padding(2, 4, 0, 4).Gap(10).AlignItems(ui.Baseline).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(k.Primary).SingleLine().Grow(1)
		ui.Text(c, subtitle).FontSize(12).TextColor(k.Muted).SingleLine()
	})
}

// floatingNoteBody is the content of a floating window: the note's Markdown in the rich text editor, with the same
// autosave as the editor.
func floatingNoteBody(c *ui.Context, edit *noteEdit) {
	syncBuffer(edit)
	if _, pending := edit.sess.Conflict(); pending {
		conflictNotice(c, "Unsaved changes from another session are waiting. Open this note in Quick Notes to choose which version to keep.")
		return
	}
	doc := noteDocument(edit)
	ui.Column(c).Grow(1).Padding(8, 4).Children(func() {
		editor := design.MarkdownEditor(c, doc, editorPlaceholder).AutoFocus()
		if editor.Changed() {
			markdown := doc.Text().Markdown()
			edit.text, edit.docSource = markdown, markdown
			_ = edit.sess.Edit(markdown)
		}
	})
}

// noteDocument returns the rich text of a note, made from its text when it has none yet and remade when the text
// changed under it, as a palette editor's typing or a conflict choice does.
func noteDocument(edit *noteEdit) *richtext.Document {
	switch {
	case edit.doc == nil:
		edit.doc = richtext.NewDocument(richtext.ParseMarkdown(edit.text))
	case edit.docSource != edit.text:
		edit.doc.SetText(richtext.ParseMarkdown(edit.text))
	}
	edit.docSource = edit.text
	return edit.doc
}
