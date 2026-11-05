package notes

import (
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// newRowTitle is the first row of the palette list, the one the Floating Notes journey starts from.
const newRowTitle = "New Note"

// editorPlaceholder and the autosave debounce are the 1.x editor's.
const (
	editorPlaceholder     = "Start with a thought, a checklist, or a tiny manifesto…"
	autosaveDebounceMilli = 400
)

// editorView is one note in the Markdown editor. Its draft autosaves 400 ms after the last keystroke and resolves a
// stored draft that conflicts with the saved note. The editor's actions are ⌘N, ⌘O and Delete note, and ⌘K lists them.
func editorView(ctx api.Context, note api.Note, mode string) api.View {
	newNote := api.Run(refNewNote, newNoteTitle, map[string]string{argMode: mode, argNav: navReplace})
	newNote.Shortcut = "Command+N"
	allNotes := api.Run(refAllNotes, "All notes", map[string]string{argMode: mode, argNav: navReplace})
	allNotes.Shortcut = "Command+O"
	remove := api.Run(refDeleteShown, deleteTitle, map[string]string{argID: note.ID, argMode: mode, argNav: navReplace})
	remove.Style = "destructive"
	autosave := api.Run(refAutosave, "Autosave note", map[string]string{argID: note.ID})
	conflict := &api.ActionRef{ID: refConflict, Args: map[string]string{argID: note.ID, argMode: mode}}
	return api.View{
		ID:       "floating-note:" + note.ID,
		Kind:     api.ViewEditor,
		Title:    noteTitle(note),
		Subtitle: "Markdown · Edited " + relativeTime(ctx, note),
		Editor: &api.EditorSpec{
			Content: note.Markdown, Format: "markdown", TitleFromLine: true, Placeholder: editorPlaceholder,
			Draft: &api.DraftOptions{
				Key: note.ID, Version: versionOf(note), DebounceMs: autosaveDebounceMilli,
				Autosave: &autosave, OnConflict: conflict,
			},
		},
		Actions:     []api.Action{newNote, allNotes, remove},
		ActionPanel: api.Panel(api.Section("Notes", newNote, allNotes), api.Section("This note", remove)),
		FooterHints: editorFooterHints(),
	}
}

// editorFooterHints are the keys the editor's footer names, as 1.x's editor footer does: Back, then the New and All
// notes shortcuts.
func editorFooterHints() []api.FooterHint {
	return []api.FooterHint{
		{Key: "Escape", Title: "Back"},
		{Key: "Command+N", Title: "New"},
		{Key: "Command+O", Title: "All notes"},
	}
}

// listView is the list of notes: New note first, then the notes most recent first. In the palette, a note opens in the
// palette, and Control+F opens it in the floating window. In the window, a note replaces the window's view.
func listView(ctx api.Context, notes []api.Note, mode string) api.View {
	newNote := api.Item{
		ID: "new-note", Title: newRowTitle, Subtitle: newNoteSubtitle, Icon: "plus",
		Primary: api.Primary(api.Run(refNewNote, newNoteTitle, navArgs(mode, navPushFor(mode)))),
	}
	items := []api.Item{newNote}
	for _, n := range notes {
		items = append(items, noteItem(ctx, n, mode))
	}
	view := api.ListView(listTitle, items...)
	view.Placeholder = listPlaceholder
	view.Actions = []api.Action{newNoteAction(mode)}
	return view
}

// newNoteAction is ⌘N in the list: it makes a note in the mode the list is shown in.
func newNoteAction(mode string) api.Action {
	action := api.Run(refNewNote, newNoteTitle, navArgs(mode, navReplace))
	action.Shortcut = "Command+N"
	return action
}

// noteItem is one note's row. Its Control+F action opens the note in the floating window, which is only offered from
// the palette.
func noteItem(ctx api.Context, n api.Note, mode string) api.Item {
	remove := api.Run(refDeleteFromList, deleteTitle, map[string]string{argID: n.ID, argMode: mode})
	remove.Style = "destructive"
	item := api.Item{
		ID: "note:" + n.ID, Title: noteTitle(n), Subtitle: notePreview(n), Icon: "file-text",
		Accessories: []api.Accessory{{Text: relativeTime(ctx, n)}},
		Primary:     api.Primary(api.Run(refOpenNote, "Open note", navArgs(mode, navPushFor(mode), argID, n.ID))),
		Actions:     []api.Action{remove},
	}
	if mode == modePalette {
		openWindow := api.Run(refOpenInWindow, openInWindowTitle, map[string]string{argID: n.ID})
		openWindow.Shortcut = "Control+F"
		openWindow.DismissAfterRun = true
		item.Shortcut = "Control+F"
		item.Actions = append([]api.Action{openWindow}, item.Actions...)
	}
	return item
}

// navArgs are the args of a row or action shown in mode, that goes to nav, followed by any pairs of extra args.
func navArgs(mode, nav string, pairs ...string) map[string]string {
	args := map[string]string{argMode: mode}
	if nav != "" {
		args[argNav] = nav
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		args[pairs[i]] = pairs[i+1]
	}
	return args
}

// navPushFor is the navigation a list row takes: the palette pushes the note above the list, and the window replaces
// its view.
func navPushFor(mode string) string {
	if mode == modePalette {
		return navPush
	}
	return navReplace
}

// versionOf is the saved version the draft is checked against: the note's update time in nanoseconds.
func versionOf(n api.Note) string {
	return strconv.FormatInt(n.UpdatedAt.UnixNano(), 10)
}
