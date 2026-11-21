package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/folders"
	"github.com/pablopunk/nvm/native/internal/store"
)

// legacyNotesMarkerFile is the document that records the 1.x notes were brought in. A note deleted since then stays
// deleted.
const legacyNotesMarkerFile = "legacy-notes.json"

// legacyNotesMarker is the marker document: it has a file once the import has run.
type legacyNotesMarker struct {
	Imported bool `json:"imported"`
}

// legacyNotesStorage is the 1.x extension-storage document of Floating Notes: its "notes" key holds the notes.
type legacyNotesStorage struct {
	Notes []legacyNoteRecord `json:"notes"`
}

// legacyNoteRecord is one note as 1.x stored it, with its times in milliseconds since the epoch.
type legacyNoteRecord struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// legacyNotesPath is the 1.x Floating Notes storage file: nevermind.floating-notes.json in the 1.x app's data folder.
func legacyNotesPath(goos string) string {
	return filepath.Join(folders.ElectronUserData(goos, userHomeDir(), os.Getenv, legacyAppFolderName), "extension-storage", "nevermind.floating-notes.json")
}

// importLegacyNotes brings the 1.x notes into the store once: only while the marker has no file. A note whose id the
// store already holds is left as it is, and a note the store cannot take is skipped. The marker is written after the
// import, so a crash before it imports again and skips the notes already there. A missing 1.x file means nothing to
// bring in, and the import is tried again on the next launch.
func importLegacyNotes(fsys platform.FS, path string, notesStore *notes.Store, marker *store.Collection[legacyNotesMarker], now time.Time) {
	if notesStore == nil || notesStore.ReadOnly() || marker == nil {
		return
	}
	if _, err := fsys.Stat(marker.Path()); err == nil {
		return
	}
	raw, err := fsys.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Println("notes: the 1.x notes were not read:", err)
		}
		return
	}
	var legacy legacyNotesStorage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		log.Println("notes: the 1.x notes are not readable:", err)
		return
	}
	added := importLegacyRecords(notesStore, legacy.Notes, now)
	log.Printf("notes: %d of %d 1.x notes brought in", added, len(legacy.Notes))
	if err := marker.Update(func(m *legacyNotesMarker) { m.Imported = true }); err != nil {
		log.Println("notes: the import marker was not saved:", err)
		return
	}
	if err := marker.Flush(); err != nil {
		log.Println("notes: the import marker was not flushed:", err)
	}
}

// importLegacyRecords adds each 1.x note the store does not hold, and reports how many it added.
func importLegacyRecords(notesStore *notes.Store, records []legacyNoteRecord, now time.Time) int {
	added := 0
	for _, record := range records {
		ok, err := notesStore.Import(legacyNoteOf(record, now))
		if errors.Is(err, notes.ErrLimit) {
			break
		}
		if err != nil {
			log.Println("notes: a 1.x note was skipped:", err)
			continue
		}
		if ok {
			added++
		}
	}
	return added
}

// legacyNoteOf is the store's note for a 1.x record. A record without times takes now.
func legacyNoteOf(record legacyNoteRecord, now time.Time) notes.Note {
	updated := millisTime(record.UpdatedAt, now)
	return notes.Note{ID: record.ID, Markdown: record.Content, CreatedAt: millisTime(record.CreatedAt, updated), UpdatedAt: updated}
}

// millisTime is the time of a 1.x timestamp, or fallback when the record has none.
func millisTime(ms int64, fallback time.Time) time.Time {
	if ms == 0 {
		return fallback
	}
	return time.UnixMilli(ms).UTC()
}
