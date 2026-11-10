package protocol

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// NoteGetResult answers a note lookup: the note, and whether the store holds it.
type NoteGetResult struct {
	Note  api.Note `json:"note"`
	Found bool     `json:"found"`
}

// NoteUpdateResult answers a save: the note as it is now, and whether a newer saved version refused the write.
type NoteUpdateResult struct {
	Note     api.Note `json:"note"`
	Conflict bool     `json:"conflict"`
}

// NoteUpdateArgs saves markdown as the note's text, on top of the saved version SavedAt.
type NoteUpdateArgs struct {
	ID       string    `json:"id"`
	Markdown string    `json:"markdown"`
	SavedAt  time.Time `json:"savedAt"`
}
