package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// notesView is Quick Notes: a row that starts a note, then every note newest first. The detail pane shows the
// Markdown of the chosen note. Filtering matches the text of the notes, as the 1.x list did.
func (p *App) notesView() *palette.View {
	return &palette.View{
		Title: "Quick Notes", Placeholder: "Search Quick Notes",
		EmptyTitle: "A clear space for your next idea.", EmptySub: "Create a Markdown note and keep it close at hand.",
		Search: func(q string) []search.Item {
			if p.notes == nil {
				return nil
			}
			out := []search.Item{{Title: "New Note", Subtitle: "Start a fresh Markdown note", Icon: "plus", Actions: []search.Action{{Title: "Create", Run: func(h search.Host) { p.openNewNote(h) }}}}}
			for _, n := range p.noteMatches(q) {
				out = append(out, p.noteItem(n))
			}
			return out
		},
	}
}

// noteMatches returns the notes whose text contains the query, all of them for a blank query.
func (p *App) noteMatches(q string) []notes.Note {
	all := p.notes.List()
	needle := strings.ToLower(strings.TrimSpace(q))
	if needle == "" {
		return all
	}
	var out []notes.Note
	for _, n := range all {
		if strings.Contains(strings.ToLower(n.Markdown), needle) {
			out = append(out, n)
		}
	}
	return out
}

// noteItem is one note in the list: its title, its preview and how long ago it was saved.
func (p *App) noteItem(n notes.Note) search.Item {
	id, title, text := n.ID, n.Title(), n.Markdown
	it := search.Item{
		ID: id, Title: title, Subtitle: notes.Preview(text), Accessory: notes.RelativeTime(clock(), n.UpdatedAt),
		Icon: "sticky-note", Tone: "yellow", TintTitle: true,
	}
	it.Actions = []search.Action{
		{Title: "Open", Run: func(h search.Host) { p.openNoteFromList(h, id) }},
		{Title: "Open as Floating Window", Label: "⌃F", Mods: mod, Key: ui.KeyF, Run: func(h search.Host) { h.Hide(); p.openFloatingNote(id) }},
		{Title: "Copy Text", Label: "⌃C", Mods: mod, Key: ui.KeyC, Run: func(h search.Host) { mygo.Clipboard.WriteText(text); h.Hide() }},
		{Title: "Delete", Label: "⌃⌫", Mods: mod, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(p.confirmDeleteNote(id, title))
		}},
	}
	it.Detail = func(c *ui.Context, _ search.Host) { noteDetail(c, p, n) }
	return it
}

// noteDetail draws the preview pane of a note: its Markdown, then its size and when it was saved.
func noteDetail(c *ui.Context, p *App, n notes.Note) {
	t := design.Tokens(c)
	ui.Scroll(c).Grow(1).Children(func() {
		if strings.TrimSpace(n.Markdown) == "" {
			ui.Text(c, "Nothing written yet.").FontSize(13).TextColor(t.Tertiary)
			return
		}
		replyMarkdown(c, n.Markdown, false, &p.md)
	})
	ui.Divider(c)
	info := fmt.Sprintf("%s · %d chars", notes.RelativeTime(clock(), n.UpdatedAt), len([]rune(n.Markdown)))
	ui.Text(c, info).FontSize(11).TextColor(t.Tertiary)
}

// openNewNote starts a note from the list or the root and opens its editor above the list.
func (p *App) openNewNote(h search.Host) {
	if p.notes == nil {
		h.Toast("Quick Notes could not be opened")
		return
	}
	n, err := p.notes.Create(clock())
	if err != nil {
		h.Toast(noteErrorMessage(err))
		return
	}
	p.openNoteFromList(h, n.ID)
}

// openNoteFromList opens a note's editor above the current view. A note with a conflict also shows the choices
// at once, so the user sees them before any edit.
func (p *App) openNoteFromList(h search.Host, id string) {
	view, edit := p.noteEditorView(id)
	if view == nil {
		h.Toast("This note no longer exists")
		return
	}
	h.Push(view)
	if _, pending := edit.sess.Conflict(); pending {
		h.Push(p.conflictView(edit))
	}
}

// newNote is the root command: it creates a note and opens its editor on the palette.
func (p *App) newNote() {
	if p.notes == nil {
		p.Toast("Quick Notes could not be opened")
		return
	}
	n, err := p.notes.Create(clock())
	if err != nil {
		p.Toast(noteErrorMessage(err))
		return
	}
	view, _ := p.noteEditorView(n.ID)
	if view != nil {
		p.Push(view)
	}
}

// confirmDeleteNote asks before a note is deleted: the palette's confirmation, not a native dialog.
func (p *App) confirmDeleteNote(id, title string) *palette.View {
	return palette.Confirm(fmt.Sprintf("Delete “%s”", title), "This cannot be undone.", func() { p.deleteNote(id) })
}

// deleteNote removes a note. An editor or window showing it is closed first, so its pending text is saved and
// then removed with the note; the draft goes with it.
func (p *App) deleteNote(id string) {
	if edit, ok := p.edits[id]; ok {
		window := edit.window
		edit.window = nil
		if window != nil {
			window.Close()
		}
		_ = edit.sess.Close()
		delete(p.edits, id)
	}
	if err := p.notes.Delete(id); err != nil {
		p.Toast(noteErrorMessage(err))
		return
	}
	p.Toast("Note deleted")
}

// noteErrorMessage says in the palette's words why a note could not be changed.
func noteErrorMessage(err error) string {
	switch {
	case errors.Is(err, notes.ErrLimit):
		return "Quick Notes is full: delete a note to make room"
	case errors.Is(err, store.ErrReadOnly):
		return "Quick Notes are read-only in this window"
	}
	return "Could not change the note: " + err.Error()
}
