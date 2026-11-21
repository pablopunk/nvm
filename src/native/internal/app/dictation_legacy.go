package app

import (
	"path/filepath"

	dictbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/dictation"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/dictation/history"
	"github.com/pablopunk/nvm/native/internal/store"
)

// The documents the app kept the dictation choices and transcripts in, before the dictation built-in kept its own.
const (
	dictationSettingsFile = "dictation.json"
	dictationHistoryFile  = "dictation-history.json"
)

// dictationHistoryDoc is the document the transcripts were kept in.
type dictationHistoryDoc struct {
	Entries []history.Entry `json:"entries"`
}

// loadLegacyDictation reads the choices and transcripts the app kept, for the built-in to answer until it saves its own.
// A document that cannot be read is left out, so the built-in starts from its defaults.
func loadLegacyDictation(st *store.Store) dictbuiltin.Legacy {
	var legacy dictbuiltin.Legacy
	if st == nil {
		return legacy
	}
	if settings, err := store.Doc(st, dictationSettingsFile, 1, func() dictbuiltin.Settings { return dictbuiltin.Settings{} }).Load(); err == nil {
		legacy.Settings = &settings
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
