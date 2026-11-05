package api

import (
	"strings"
	"time"
)

// Notes reads and writes the user's Markdown notes, the ones the Notes built-in shows. It needs the notes capability.
type Notes interface {
	List() ([]Note, error)
	Get(id string) (Note, bool, error)
	Create() (Note, error)
	// Recent returns the most recent note, creating one when there is none.
	Recent() (Note, error)
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

// Refs the conflict view's rows run. The extension's own RunAction answers them.
const (
	NoteConflictRestoreRef = "note-conflict-restore"
	NoteConflictKeepRef    = "note-conflict-keep"
)

// NoteConflictView is the view an extension shows when Update met a newer saved version: the unsaved text it kept, the
// saved text, and Compare, which pushes a preview of the two. Restore unsaved changes runs NoteConflictRestoreRef and
// Keep saved version runs NoteConflictKeepRef; the extension decides what each one writes.
func NoteConflictView(saved Note, unsaved string, compareMarkdown string) View {
	return ListView("Unsaved changes found",
		Item{ID: "note-conflict-restore", Title: "Restore unsaved changes", Subtitle: firstLine(unsaved),
			Primary: Primary(Run(NoteConflictRestoreRef, "Restore unsaved changes", nil))},
		Item{ID: "note-conflict-keep", Title: "Keep saved version", Subtitle: firstLine(saved.Markdown),
			Primary: Primary(Run(NoteConflictKeepRef, "Keep saved version", nil))},
		Item{ID: "note-conflict-compare", Title: "Compare", Subtitle: "See the unsaved and saved text side by side",
			Primary: Primary(PushView("Compare", PreviewView("Compare", compareMarkdown)))},
	)
}

// firstLine is the first non-empty line of text, the subtitle a conflict row shows.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
