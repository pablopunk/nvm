package api

import "time"

// noteConflictTitle and noteConflictSubtitle are the conflict view's title and the line under it, as 1.x shows them.
const (
	noteConflictTitle    = "Unsaved changes found"
	noteConflictSubtitle = "This note has unsaved changes from another session."
)

// Arg keys the conflict view's rows carry back to the extension.
const (
	argNoteID   = "id"
	argNoteText = "text"
)

// Notes reads and writes the user's Markdown notes, the ones the Notes built-in shows. It needs the notes capability.
type Notes interface {
	List() ([]Note, error)
	Get(id string) (Note, bool, error)
	Create() (Note, error)
	// Recent returns the most recent note, creating one when there is none.
	Recent() (Note, error)
	// Search finds the notes whose text holds every word of query, ignoring case, best matches first: notes whose title
	// holds the words come before the others, and newer before older. An empty query lists the most recent notes. limit
	// is at most 50 and 20 when it is zero. A match carries a title and a snippet, never the whole text: read it with
	// Get.
	Search(query string, limit int) ([]NoteMatch, error)
	// CreateWith creates a note that holds markdown. Prefer it to Create followed by Update.
	CreateWith(markdown string) (Note, error)
	// Update saves markdown when the saved version is still updatedAt. A newer one is a conflict: nothing is written,
	// and the saved note comes back with true.
	Update(id, markdown string, updatedAt time.Time) (Note, bool, error)
	// Delete removes a note after the user confirms.
	Delete(id string) error
}

// Note is one Markdown note. UpdatedAt is its saved version: pass it back to Update.
type Note struct {
	ID        string    `json:"id"`
	Markdown  string    `json:"markdown"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NoteMatch is one note a search found: its id, title (the first line, without heading marks), a snippet of the text
// around the first hit, and its saved version.
type NoteMatch struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Snippet   string    `json:"snippet"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Refs the conflict view's rows run. The extension's own RunAction answers them.
const (
	NoteConflictRestoreRef = "note-conflict-restore"
	NoteConflictKeepRef    = "note-conflict-keep"
)

// NoteConflictView is the view shown when Update met a newer saved version, as 1.x shows it. Restore unsaved changes
// runs NoteConflictRestoreRef with the id and the unsaved text under "text"; Keep saved version runs NoteConflictKeepRef.
func NoteConflictView(saved Note, unsaved string, args map[string]string) View {
	restore := withArgs(args, map[string]string{argNoteID: saved.ID, argNoteText: unsaved})
	keep := withArgs(args, map[string]string{argNoteID: saved.ID})
	view := ListView(noteConflictTitle,
		Item{ID: "note-conflict-restore", Title: "Restore unsaved changes", Subtitle: NotePreview(unsaved), Icon: "rotate-ccw",
			Primary: Primary(Run(NoteConflictRestoreRef, "Restore unsaved changes", restore))},
		Item{ID: "note-conflict-keep", Title: "Keep saved version", Subtitle: NotePreview(saved.Markdown), Icon: "check",
			Primary: Primary(Run(NoteConflictKeepRef, "Keep saved version", keep))},
	)
	view.Subtitle = noteConflictSubtitle
	return view
}

// withArgs returns a copy of base with the pairs of extra written over it.
func withArgs(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
