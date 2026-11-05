package notes

import (
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Document name and schema version. The document starts at version 1 (store/MIGRATING.md).
const (
	notesFile     = "notes.json"
	schemaVersion = 1
)

// ErrNotFound is returned for a note id that the store does not hold.
var ErrNotFound = errors.New("notes: no such note")

// ErrLimit is returned by Create when the store holds MaxNotes notes already.
var ErrLimit = errors.New("notes: the limit of 500 notes is reached; delete one to make room")

type notesDocument struct {
	Notes []Note `json:"notes"`
}

// Store holds the notes. It is created over a store.Store that the caller opens and closes; New registers the
// notes document on it. A read-only Store (the directory's lease is held by another instance) reads everything and
// writes nothing. A Store is safe for concurrent use.
type Store struct {
	st       *store.Store
	clock    platform.Clock
	readOnly bool
	notes    *store.Collection[notesDocument]

	mu       sync.Mutex
	sequence uint64
}

// New registers notes.json on st. readOnly must be true when st was opened without the lease.
func New(st *store.Store, clock platform.Clock, readOnly bool) *Store {
	return &Store{
		st:       st,
		clock:    clock,
		readOnly: readOnly,
		notes:    store.Doc(st, notesFile, schemaVersion, func() notesDocument { return notesDocument{} }),
	}
}

// ReadOnly reports whether writes are refused.
func (s *Store) ReadOnly() bool {
	return s.readOnly
}

// List returns the notes, most recently saved first. Malformed entries are left out.
func (s *Store) List() []Note {
	out := s.usableNotes()
	slices.SortStableFunc(out, func(a, b Note) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return out
}

// Get returns the note with the given id.
func (s *Store) Get(id string) (Note, bool) {
	notes := s.usableNotes()
	i := slices.IndexFunc(notes, func(n Note) bool { return n.ID == id })
	if i < 0 {
		return Note{}, false
	}
	return notes[i], true
}

// MostRecentOrCreate returns the note saved most recently, or creates an empty one when there is none.
func (s *Store) MostRecentOrCreate(now time.Time) (Note, error) {
	if notes := s.List(); len(notes) > 0 {
		return notes[0], nil
	}
	return s.Create(now)
}

// Create adds an empty note created at now and returns it.
func (s *Store) Create(now time.Time) (Note, error) {
	if s.readOnly {
		return Note{}, store.ErrReadOnly
	}
	if len(s.usableNotes()) >= MaxNotes {
		return Note{}, ErrLimit
	}
	note := Note{ID: s.nextID(now), CreatedAt: now, UpdatedAt: now}
	if err := s.notes.Update(func(d *notesDocument) { d.Notes = append([]Note{note}, d.Notes...) }); err != nil {
		return Note{}, err
	}
	return note, nil
}

// Delete removes a note. The caller confirms with the user first.
func (s *Store) Delete(id string) error {
	if s.readOnly {
		return store.ErrReadOnly
	}
	if _, ok := s.Get(id); !ok {
		return ErrNotFound
	}
	return s.notes.Update(func(d *notesDocument) {
		d.Notes = slices.DeleteFunc(d.Notes, func(n Note) bool { return n.ID == id })
	})
}

// SaveIfUnchanged writes markdown as the note's saved text at now when the note is still at the saved version
// savedAt. A newer saved version is not overwritten: the call returns that note with conflict set, and writes nothing.
func (s *Store) SaveIfUnchanged(id, markdown string, savedAt, now time.Time) (note Note, conflict bool, err error) {
	current, ok := s.Get(id)
	if !ok {
		return Note{}, false, ErrNotFound
	}
	if !current.UpdatedAt.Equal(savedAt) {
		return current, true, nil
	}
	saved, err := s.saveText(id, markdown, now)
	return saved, false, err
}

// saveText writes markdown as the note's saved text at now and returns the note as saved.
func (s *Store) saveText(id, markdown string, now time.Time) (Note, error) {
	if s.readOnly {
		return Note{}, store.ErrReadOnly
	}
	current, ok := s.Get(id)
	if !ok {
		return Note{}, ErrNotFound
	}
	text, saved := truncateText(markdown), nextSavedTime(now, current.Version())
	if err := s.updateNote(id, func(n *Note) { n.Markdown, n.UpdatedAt = text, saved }); err != nil {
		return Note{}, err
	}
	current.Markdown, current.UpdatedAt = text, saved
	return current, nil
}

// usableNotes returns the notes that are well formed, at most MaxNotes of them, in stored order.
func (s *Store) usableNotes() []Note {
	doc, _ := s.notes.Load()
	out := make([]Note, 0, len(doc.Notes))
	for _, n := range doc.Notes {
		if validNoteID(n.ID) && len(out) < MaxNotes {
			out = append(out, n)
		}
	}
	return out
}

// updateNote changes the note with the given id.
func (s *Store) updateNote(id string, change func(*Note)) error {
	if s.readOnly {
		return store.ErrReadOnly
	}
	if _, ok := s.Get(id); !ok {
		return ErrNotFound
	}
	return s.notes.Update(func(d *notesDocument) {
		for i := range d.Notes {
			if d.Notes[i].ID == id {
				change(&d.Notes[i])
			}
		}
	})
}

// nextID returns a fresh note id for a note created at now.
func (s *Store) nextID(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	return newNoteID(now, s.sequence)
}
