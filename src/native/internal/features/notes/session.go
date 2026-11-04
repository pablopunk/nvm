package notes

import (
	"errors"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// Errors of an editing session.
var (
	// ErrNoConflict is returned by a resolution when the note has no unsaved changes waiting.
	ErrNoConflict = errors.New("notes: there are no unsaved changes to resolve")
	// ErrStaleConflict is returned when the saved note changed since the conflict was shown. The conflict is
	// refreshed against the new saved text, so the user sees it again before choosing.
	ErrStaleConflict = errors.New("notes: the saved note changed; review the unsaved changes again")
	// ErrConflictPending is returned by Edit while a conflict waits for a choice.
	ErrConflictPending = errors.New("notes: resolve the unsaved changes before editing")
	// ErrSessionClosed is returned by Edit after Close.
	ErrSessionClosed = errors.New("notes: the editor is closed")
)

// Status words the editor shows next to the note.
const (
	StatusSaved    = "Saved"
	StatusSaving   = "Saving…"
	StatusRetrying = "Not saved yet; retrying"
	StatusReadOnly = "Read-only: another window has these notes open"
	StatusConflict = "Unsaved changes found"
)

// Conflict is unsaved text that was typed on top of a saved version the note no longer has.
type Conflict struct {
	Saved Note
	Draft Draft
}

// Diff compares the saved text with the unsaved text, line by line.
func (c Conflict) Diff() []DiffLine {
	return Diff(c.Saved.Markdown, c.Draft.Content)
}

// Session edits one note. Keystrokes go in with Edit, which records a draft at once; the draft is written by the
// store 300 ms after the last keystroke. Autosave writes the note 400 ms after the last keystroke, and the draft
// is removed only after the note is saved, so a crash between the two leaves the draft to be recovered.
//
// A session belongs to the UI thread for its edits; autosave runs on a goroutine of its own and takes the same
// lock, so every method is safe to call from any goroutine.
type Session struct {
	store   *Store
	clock   platform.Clock
	id      string
	onSaved func()
	done    chan struct{}

	mu        sync.Mutex
	base      int64
	saved     string
	text      string
	conflict  *Conflict
	recovered bool
	closed    bool
	armed     bool
	deadline  time.Time
	lastErr   error
}

// OpenSession opens a note for editing and resolves what the draft store holds for it. A draft based on the
// saved version is recovered and autosaved; a draft with the same text as the note is dropped; a draft based on
// another version, with different text, is a conflict that waits for KeepSaved or KeepDraft. onSaved, when not
// nil, runs after each autosave so the UI can redraw; it must not block.
func (s *Store) OpenSession(id string, onSaved func()) (*Session, error) {
	note, ok := s.Get(id)
	if !ok {
		return nil, ErrNotFound
	}
	sess := &Session{store: s, clock: s.clock, id: id, onSaved: onSaved, done: make(chan struct{}), base: note.Version(), saved: note.Markdown, text: note.Markdown}
	if s.readOnly {
		return sess, nil
	}
	draft, ok := s.draftFor(id)
	switch {
	case !ok:
	case draft.Base == note.Version():
		sess.text = draft.Content
		sess.recovered = draft.Content != note.Markdown
		if sess.recovered {
			sess.mu.Lock()
			sess.touchLocked()
			sess.mu.Unlock()
		}
	case draft.Content == note.Markdown:
		if err := s.removeDraft(id); err != nil {
			sess.lastErr = err
		}
	default:
		sess.conflict = &Conflict{Saved: note, Draft: draft}
	}
	return sess, nil
}

// ID is the note the session edits.
func (s *Session) ID() string {
	return s.id
}

// Text is the text the editor shows: the saved text, the recovered draft, or the saved text while a conflict
// waits.
func (s *Session) Text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text
}

// Recovered reports whether the session opened on unsaved text from a draft.
func (s *Session) Recovered() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recovered
}

// Conflict returns the unsaved changes waiting for a choice, and whether there are any.
func (s *Session) Conflict() (Conflict, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conflict == nil {
		return Conflict{}, false
	}
	return *s.conflict, true
}

// Status is the words the editor shows: saved, saving, retrying after a failed save, read-only, or a conflict.
func (s *Session) Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.store.readOnly:
		return StatusReadOnly
	case s.conflict != nil:
		return StatusConflict
	case s.lastErr != nil:
		return StatusRetrying
	case s.text == s.saved:
		return StatusSaved
	}
	return StatusSaving
}

// Edit replaces the text. It records a draft and schedules the autosave. Edits wait while a conflict is open.
func (s *Session) Edit(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.closed:
		return ErrSessionClosed
	case s.store.readOnly:
		return store.ErrReadOnly
	case s.conflict != nil:
		return ErrConflictPending
	}
	text = truncateText(text)
	if text == s.text {
		return nil
	}
	s.text = text
	err := s.store.putDraft(Draft{NoteID: s.id, Base: s.base, Content: text, Edited: s.clock.Now()})
	s.touchLocked()
	return err
}

// KeepSaved discards the unsaved changes and shows the saved text.
func (s *Session) KeepSaved() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conflict == nil {
		return ErrNoConflict
	}
	current, ok := s.store.Get(s.id)
	if !ok {
		return ErrNotFound
	}
	if current.Version() != s.conflict.Saved.Version() {
		s.conflict = &Conflict{Saved: current, Draft: s.conflict.Draft}
		return ErrStaleConflict
	}
	if err := s.store.removeDraft(s.id); err != nil {
		return err
	}
	s.text, s.saved, s.base = current.Markdown, current.Markdown, current.Version()
	s.conflict = nil
	return nil
}

// KeepDraft keeps the unsaved changes: they become the text of the editor and are saved as the note's new
// version by autosave.
func (s *Session) KeepDraft() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conflict == nil {
		return ErrNoConflict
	}
	current, ok := s.store.Get(s.id)
	if !ok {
		return ErrNotFound
	}
	if current.Version() != s.conflict.Saved.Version() {
		s.conflict = &Conflict{Saved: current, Draft: s.conflict.Draft}
		return ErrStaleConflict
	}
	draft := Draft{NoteID: s.id, Base: current.Version(), Content: s.conflict.Draft.Content, Edited: s.clock.Now()}
	if err := s.store.putDraft(draft); err != nil {
		return err
	}
	s.text, s.saved, s.base = draft.Content, current.Markdown, current.Version()
	s.conflict = nil
	s.touchLocked()
	return nil
}

// Close saves what is pending and ends the session. Pending text with a conflict is left as the draft, so it is
// offered again the next time the note opens.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.saveLocked()
	s.closed = true
	close(s.done)
	return s.lastErr
}

// touchLocked moves the autosave deadline one delay after now and starts the waiter when none is running.
// The caller holds s.mu.
func (s *Session) touchLocked() {
	s.deadline = s.clock.Now().Add(AutosaveDelay)
	if s.armed {
		return
	}
	s.armed = true
	go s.waitAutosave(s.clock.After(AutosaveDelay))
}

// waitAutosave sleeps until the deadline passes and saves. A keystroke that moved the deadline re-arms it.
func (s *Session) waitAutosave(timer <-chan time.Time) {
	for {
		select {
		case <-s.done:
			return
		case <-timer:
		}
		s.mu.Lock()
		if s.closed {
			s.armed = false
			s.mu.Unlock()
			return
		}
		if remaining := s.deadline.Sub(s.clock.Now()); remaining > 0 {
			timer = s.clock.After(remaining)
			s.mu.Unlock()
			continue
		}
		s.armed = false
		s.saveLocked()
		s.mu.Unlock()
		if s.onSaved != nil {
			s.onSaved()
		}
		return
	}
}

// saveLocked writes the text as the note's saved text, flushes the notes document, and only then drops the draft
// and flushes the drafts document. The order is the crash guarantee: the draft is never gone from disk before the
// note it holds is. A failed write keeps the draft and the error, which Status reports. The caller holds s.mu.
func (s *Session) saveLocked() {
	if s.store.readOnly || s.conflict != nil || s.text == s.saved {
		return
	}
	note, err := s.store.saveText(s.id, s.text, s.clock.Now())
	if err == nil {
		err = s.store.flushNotes()
	}
	if err != nil {
		s.lastErr = err
		return
	}
	s.saved, s.base, s.recovered = note.Markdown, note.Version(), false
	s.lastErr = nil
	if err := s.store.removeDraft(s.id); err != nil {
		s.lastErr = err
		return
	}
	if err := s.store.flushDrafts(); err != nil {
		s.lastErr = err
	}
}
