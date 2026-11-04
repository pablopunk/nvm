package app

import (
	"errors"
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

const editorPlaceholder = "Start with a thought, a checklist, or a tiny manifesto…"

// noteEdit is one note being edited. Its session owns the text; the buffer is what the text area types into. The
// palette editors and the floating window of the note share it, so both show the same text and the same conflict.
// The UI thread owns it; the session is safe to call from the autosave goroutine as well.
type noteEdit struct {
	id      string
	sess    *notes.Session
	text    string
	editors int
	window  *mygo.Window
}

// acquireEdit returns the shared edit of a note, opening its session on first use. It reports false when the note
// does not exist.
func (p *App) acquireEdit(id string) (*noteEdit, bool) {
	if edit, ok := p.edits[id]; ok {
		return edit, true
	}
	sess, err := p.notes.OpenSession(id, p.redrawAfterSave)
	if err != nil {
		return nil, false
	}
	if p.edits == nil {
		p.edits = map[string]*noteEdit{}
	}
	edit := &noteEdit{id: id, sess: sess, text: sess.Text()}
	p.edits[id] = edit
	return edit, true
}

// releaseEdit ends the session once no palette editor and no window shows the note. Ending it saves what is
// pending, or keeps the draft when a conflict waits.
func (p *App) releaseEdit(edit *noteEdit) {
	if edit.editors > 0 || edit.window != nil {
		return
	}
	if p.edits[edit.id] == edit {
		delete(p.edits, edit.id)
	}
	_ = edit.sess.Close()
}

// redrawAfterSave redraws the palette when an autosave lands, so its status line changes. It is safe from any
// goroutine; before the window exists (tests) it does nothing.
func (p *App) redrawAfterSave() {
	if p.panel != nil {
		p.panel.Update(func() {})
	}
}

// syncBuffer adopts the session's text when it changed under the buffer, which only a conflict resolution does.
func syncBuffer(edit *noteEdit) {
	if text := edit.sess.Text(); text != edit.text {
		edit.text = text
	}
}

// noteEditorView is the editor of one note: the Markdown text, the autosave status in the footer, and the
// conflict choices while unsaved changes wait. It returns nil when the note does not exist.
func (p *App) noteEditorView(id string) (*palette.View, *noteEdit) {
	if p.notes == nil {
		return nil, nil
	}
	edit, ok := p.acquireEdit(id)
	if !ok {
		return nil, nil
	}
	edit.editors++
	view := &palette.View{
		Title: "Note", HideInput: true, Hint: "esc Back   ⌃N New   ⌃F Float window",
		TitleFn: func() string { return notes.Title(edit.text) },
		OnLeave: func() {
			edit.editors--
			p.releaseEdit(edit)
		},
		OnKey: func(c *ui.Context, h search.Host) bool { return p.noteEditorKeys(c, h, edit) },
		Body: func(c *ui.Context, _ search.Host) {
			syncBuffer(edit)
			if _, pending := edit.sess.Conflict(); pending {
				conflictNotice(c, "Unsaved changes from another session are waiting. Press Enter to choose which version to keep.")
				return
			}
			ui.Column(c).Grow(1).Children(func() {
				ta := ui.TextArea(c, &edit.text).Grow(1).Margin(12).Padding(10).FontSize(15).Placeholder(editorPlaceholder).AutoFocus()
				if ta.Changed() {
					_ = edit.sess.Edit(edit.text)
				}
				autosaveStatus(c, edit.sess.Status())
			})
		},
	}
	return view, edit
}

// autosaveStatus draws the save state under the text: saved, saving, retrying, or read-only.
func autosaveStatus(c *ui.Context, status string) {
	t := design.Tokens(c)
	ui.Text(c, status).FontSize(11).TextColor(t.Tertiary).Margin(12)
}

// noteEditorKeys are the editor's shortcuts. They run before the text area sees the keys.
func (p *App) noteEditorKeys(c *ui.Context, h search.Host, edit *noteEdit) bool {
	_, pending := edit.sess.Conflict()
	switch {
	case pending && c.Shortcut(0, ui.KeyEnter):
		h.Push(p.conflictView(edit))
	case c.Shortcut(mod, ui.KeyN):
		p.openNewNote(h)
	case c.Shortcut(mod, ui.KeyF):
		h.Hide()
		p.openFloatingNote(edit.id)
	case c.Shortcut(mod, ui.KeyBackspace):
		h.Push(p.confirmDeleteNote(edit.id, notes.Title(edit.text)))
	default:
		return false
	}
	return true
}

// conflictNotice draws the line that replaces the text area while a conflict waits.
func conflictNotice(c *ui.Context, text string) {
	t := design.Tokens(c)
	ui.Text(c, text).FontSize(13).TextColor(t.Secondary)
}

// conflictView lists the choices for unsaved changes that were typed on top of an older version of the note.
// Keeping either version closes the list and returns to the editor; comparing opens the diff.
func (p *App) conflictView(edit *noteEdit) *palette.View {
	return &palette.View{
		Title: "Unsaved changes found", Placeholder: "This note has unsaved changes from another session",
		EmptyTitle: "No unsaved changes.", EmptySub: "The note is up to date.",
		Search: func(string) []search.Item {
			conflict, ok := edit.sess.Conflict()
			if !ok {
				return nil
			}
			summary := notes.Summarize(conflict.Diff())
			now := clock()
			return []search.Item{
				{Title: "Keep saved version", Subtitle: "Saved " + notes.RelativeTime(now, conflict.Saved.UpdatedAt), Icon: "file-text", Tone: "blue", Actions: []search.Action{
					{Title: "Keep saved version", Run: func(h search.Host) { p.resolveConflict(h, edit, edit.sess.KeepSaved) }},
				}},
				{Title: "Keep draft", Subtitle: "Unsaved text, edited " + notes.RelativeTime(now, conflict.Draft.Edited), Icon: "sticky-note", Tone: "yellow", Actions: []search.Action{
					{Title: "Keep draft", Run: func(h search.Host) { p.resolveConflict(h, edit, edit.sess.KeepDraft) }},
				}},
				{Title: "Resolve unsaved changes", Subtitle: fmt.Sprintf("Compare: %s added, %s removed", plural(summary.Added, "line"), plural(summary.Removed, "line")), Icon: "scan-text", Tone: "blue", Actions: []search.Action{
					{Title: "Compare", Run: func(h search.Host) { h.Push(p.conflictDiffView(edit)) }},
				}},
			}
		},
	}
}

// resolveConflict applies a choice and returns to the editor, which shows the text the choice left. A refused
// choice (the saved note changed meanwhile) stays on the list, which now shows the newer saved text.
func (p *App) resolveConflict(h search.Host, edit *noteEdit, resolve func() error) {
	if err := resolve(); err != nil {
		h.Toast(conflictErrorMessage(err))
		h.Refresh()
		return
	}
	edit.text = edit.sess.Text()
	h.Pop()
}

// conflictErrorMessage says why a choice could not be applied.
func conflictErrorMessage(err error) string {
	switch {
	case errors.Is(err, notes.ErrStaleConflict):
		return "The note changed in another window; review the unsaved changes again"
	case errors.Is(err, notes.ErrNoConflict):
		return "There are no unsaved changes left to resolve"
	}
	return "Could not resolve the unsaved changes: " + err.Error()
}

// conflictDiffView shows the saved and the unsaved text line by line: removed lines in the danger color, added
// lines in the strong text color, unchanged lines dimmed. Escape returns to the choices.
func (p *App) conflictDiffView(edit *noteEdit) *palette.View {
	return &palette.View{
		Title: "Compare unsaved changes", HideInput: true, Hint: "esc Back",
		Body: func(c *ui.Context, _ search.Host) {
			conflict, ok := edit.sess.Conflict()
			if !ok {
				return
			}
			t := design.Tokens(c)
			ui.Scroll(c).Grow(1).Children(func() {
				for _, line := range conflict.Diff() {
					ui.Text(c, diffMark(line.Op)+line.Text).FontSize(12).TextColor(diffColor(t, line.Op))
				}
			})
		},
	}
}

// plural counts a noun: "1 line", "2 lines".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// diffMark is the column a diff line starts with.
func diffMark(op notes.DiffOp) string {
	switch op {
	case notes.Add:
		return "+ "
	case notes.Remove:
		return "- "
	}
	return "  "
}

// diffColor is the text color of a diff line.
func diffColor(t *design.TokenSet, op notes.DiffOp) ui.Color {
	switch op {
	case notes.Add:
		return t.Strong
	case notes.Remove:
		return t.Danger
	}
	return t.Tertiary
}
