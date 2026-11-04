package app

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/dictation/history"
	"github.com/pablopunk/nvm/native/internal/dictation/recordings"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/store"
)

// dictationHistoryFile holds the transcripts of finished dictations in the user's store.
const dictationHistoryFile = "dictation-history.json"

// dictationHistoryDoc is the document the transcripts are kept in.
type dictationHistoryDoc struct {
	Entries []history.Entry `json:"entries"`
}

// transcriptStore reads and changes the transcripts. Without a writable store they last for the process.
type transcriptStore interface {
	Entries() []history.Entry
	Update(change func([]history.Entry) []history.Entry) error
}

// memoryTranscripts keeps the transcripts in memory.
type memoryTranscripts struct {
	entries []history.Entry
}

func (m *memoryTranscripts) Entries() []history.Entry {
	return m.entries
}

func (m *memoryTranscripts) Update(change func([]history.Entry) []history.Entry) error {
	m.entries = change(m.entries)
	return nil
}

// storedTranscripts keeps the transcripts in the user's store.
type storedTranscripts struct {
	doc *store.Collection[dictationHistoryDoc]
}

func (s storedTranscripts) Entries() []history.Entry {
	value, err := s.doc.Load()
	if err != nil {
		return nil
	}
	return history.Clean(value.Entries)
}

func (s storedTranscripts) Update(change func([]history.Entry) []history.Entry) error {
	return s.doc.Update(func(value *dictationHistoryDoc) {
		value.Entries = change(history.Clean(value.Entries))
	})
}

// dictationTranscripts returns the transcripts store, created on first use from the user's store.
func (p *App) dictationTranscripts() transcriptStore {
	if p.transcripts == nil {
		if p.userStore == nil || p.userReadOnly {
			p.transcripts = &memoryTranscripts{}
		} else {
			p.transcripts = storedTranscripts{doc: store.Doc(p.userStore, dictationHistoryFile, 1, func() dictationHistoryDoc { return dictationHistoryDoc{} })}
		}
	}
	return p.transcripts
}

// rememberDictation adds a finished dictation's text to the history. A save that fails is logged, and the dictation
// is not affected.
func (p *App) rememberDictation(text string) {
	entry := history.Entry{ID: dictationEntryID(), Text: text, CreatedAt: time.Now().UnixMilli()}
	if err := p.dictationTranscripts().Update(func(entries []history.Entry) []history.Entry {
		return history.Add(entries, entry)
	}); err != nil {
		logx.Default().Warn("dictation.history-save-failed", logx.F{"error": err.Error()})
	}
}

// forgetTranscript removes one transcript from the history.
func (p *App) forgetTranscript(id string) {
	_ = p.dictationTranscripts().Update(func(entries []history.Entry) []history.Entry {
		return history.Remove(entries, id)
	})
}

// clearTranscripts removes every transcript from the history.
func (p *App) clearTranscripts() {
	_ = p.dictationTranscripts().Update(func([]history.Entry) []history.Entry { return nil })
}

// dictationRecordingStore returns the store of unfinished dictations, or nil when the data folder is read only.
func (p *App) dictationRecordingStore() *recordings.Store {
	if p.userReadOnly {
		return nil
	}
	return recordings.New(p.ports.FS, dictationRecordingsDir(), p.ports.Clock.Now)
}

// dictationEntryID returns a new ID for a transcript.
func dictationEntryID() string {
	return "dictation-" + time.Now().UTC().Format("20060102150405.000000000")
}
