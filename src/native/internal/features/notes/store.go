package notes

import (
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Document names and schema version. Both documents start at version 1 (store/MIGRATING.md).
const (
	notesFile     = "notes.json"
	draftsFile    = "drafts.json"
	schemaVersion = 1
)

// ErrNotFound is returned for a note id that the store does not hold.
var ErrNotFound = errors.New("notes: no such note")

// ErrLimit is returned by Create when the store holds MaxNotes notes already.
var ErrLimit = errors.New("notes: the limit of 500 notes is reached; delete one to make room")

// ErrInvalidDraft is returned by a draft write whose note id, version or text is out of bounds.
var ErrInvalidDraft = errors.New("notes: invalid draft")

// Draft is unsaved text for one note. Base is the saved version the text was typed on top of; a draft whose
// base no longer matches the saved note is a conflict, not a recovery.
type Draft struct {
	NoteID  string    `json:"noteId"`
	Base    int64     `json:"base"`
	Content string    `json:"content"`
	Edited  time.Time `json:"edited"`
}

type notesDocument struct {
	Notes []Note `json:"notes"`
}

type draftsDocument struct {
	Drafts []Draft `json:"drafts"`
}

// Store holds the notes and their drafts. It is created over a store.Store that the caller opens and closes;
// New registers the two documents on it. A read-only Store (the directory's lease is held by another instance)
// reads everything and writes nothing. A Store is safe for concurrent use.
type Store struct {
	st       *store.Store
	clock    platform.Clock
	readOnly bool
	notes    *store.Collection[notesDocument]
	drafts   *store.Collection[draftsDocument]

	mu       sync.Mutex
	sequence uint64
}

// New registers notes.json and drafts.json on st. readOnly must be true when st was opened without the lease.
func New(st *store.Store, clock platform.Clock, readOnly bool) *Store {
	return &Store{
		st:       st,
		clock:    clock,
		readOnly: readOnly,
		notes:    store.Doc(st, notesFile, schemaVersion, func() notesDocument { return notesDocument{} }),
		drafts:   store.Doc(st, draftsFile, schemaVersion, func() draftsDocument { return draftsDocument{} }),
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

// MostRecentOrCreate returns the note saved most recently, or creates an empty one when there is none. The
// floating-window command and the window restore open it.
func (s *Store) MostRecentOrCreate(now time.Time) (Note, error) {
	if notes := s.List(); len(notes) > 0 {
		return notes[0], nil
	}
	return s.Create(now)
}

// Floating returns the notes whose windows were open when the app last ran, in list order.
func (s *Store) Floating() []Note {
	var out []Note
	for _, note := range s.List() {
		if note.Floating {
			out = append(out, note)
		}
	}
	return out
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

// Delete removes a note and its draft. The caller confirms with the user first.
func (s *Store) Delete(id string) error {
	if s.readOnly {
		return store.ErrReadOnly
	}
	if _, ok := s.Get(id); !ok {
		return ErrNotFound
	}
	if err := s.notes.Update(func(d *notesDocument) {
		d.Notes = slices.DeleteFunc(d.Notes, func(n Note) bool { return n.ID == id })
	}); err != nil {
		return err
	}
	return s.removeDraft(id)
}

// SetFloating records whether the note's window is open. Its frame is kept either way.
func (s *Store) SetFloating(id string, floating bool) error {
	return s.updateNote(id, func(n *Note) { n.Floating = floating })
}

// SetFrame records where the note's floating window is.
func (s *Store) SetFrame(id string, frame Frame) error {
	return s.updateNote(id, func(n *Note) { n.Frame = &frame })
}

// Drafts returns the drafts the store holds, malformed ones left out.
func (s *Store) Drafts() []Draft {
	doc, _ := s.drafts.Load()
	var out []Draft
	for _, d := range doc.Drafts {
		if validDraft(d) {
			out = append(out, d)
		}
	}
	return out
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

// draftFor returns the draft of a note, when there is one.
func (s *Store) draftFor(id string) (Draft, bool) {
	for _, d := range s.Drafts() {
		if d.NoteID == id {
			return d, true
		}
	}
	return Draft{}, false
}

// putDraft replaces the draft of the note it names. Malformed drafts found in the document are dropped on the
// way, so the file heals itself the next time a draft is written.
func (s *Store) putDraft(d Draft) error {
	if s.readOnly {
		return store.ErrReadOnly
	}
	if !validDraft(d) {
		return ErrInvalidDraft
	}
	return s.drafts.Update(func(doc *draftsDocument) {
		doc.Drafts = slices.DeleteFunc(doc.Drafts, func(x Draft) bool { return !validDraft(x) || x.NoteID == d.NoteID })
		doc.Drafts = append(doc.Drafts, d)
	})
}

// removeDraft drops the draft of a note; a note without one is a no-op.
func (s *Store) removeDraft(id string) error {
	if s.readOnly {
		return store.ErrReadOnly
	}
	if _, ok := s.draftFor(id); !ok {
		return nil
	}
	return s.drafts.Update(func(doc *draftsDocument) {
		doc.Drafts = slices.DeleteFunc(doc.Drafts, func(x Draft) bool { return !validDraft(x) || x.NoteID == id })
	})
}

// flushNotes writes the notes document now when it has unsaved changes.
func (s *Store) flushNotes() error {
	return s.notes.Flush()
}

// flushDrafts writes the drafts document now when it has unsaved changes.
func (s *Store) flushDrafts() error {
	return s.drafts.Flush()
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

// validDraft reports whether a draft is well formed: a note id of the right shape, a non-negative base version
// and text within the limit.
func validDraft(d Draft) bool {
	return validNoteID(d.NoteID) && d.Base >= 0 && len(d.Content) <= MaxTextBytes
}
