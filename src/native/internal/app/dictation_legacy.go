package app

import (
	"path/filepath"

	dictbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/history"
	"github.com/pablopunk/nvm/native/internal/store"
)

// dictationHistoryFile is the document the app kept the transcripts in, before the dictation built-in kept its own.
const dictationHistoryFile = "dictation-history.json"

// dictationHistoryDoc is the document the transcripts were kept in.
type dictationHistoryDoc struct {
	Entries []history.Entry `json:"entries"`
}

// loadLegacyDictation reads the transcripts the app kept, for the built-in to answer until it saves its own. A document
// that cannot be read is left out, so the built-in starts with no history.
func loadLegacyDictation(st *store.Store) dictbuiltin.Legacy {
	var legacy dictbuiltin.Legacy
	if st == nil {
		return legacy
	}
	if doc, err := store.Doc(st, dictationHistoryFile, 1, func() dictationHistoryDoc { return dictationHistoryDoc{} }).Load(); err == nil {
		legacy.History = doc.Entries
	}
	return legacy
}

// dictationRecordingsFolder holds the audio of dictations whose text is not in yet, inside the data folder.
const dictationRecordingsFolder = "dictation-recordings"

// dictationRecordingsDir is the folder the unfinished dictations are kept in.
func dictationRecordingsDir() string {
	return filepath.Join(dataDir(), dictationRecordingsFolder)
}
