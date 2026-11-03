package snippets

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// snippetsFile is the document name; the schema starts at version 1 (store/MIGRATING.md).
const (
	snippetsFile  = "snippets.json"
	schemaVersion = 1
)

type snippetsDocument struct {
	Snippets []Snippet `json:"snippets"`
}

// Store holds the snippets. It is created over a store.Store that the caller opens and closes. A read-only Store
// (another instance holds the lease) lists and matches snippets and writes nothing. A Store is safe for concurrent
// use.
type Store struct {
	clock    platform.Clock
	readOnly bool
	doc      *store.Collection[snippetsDocument]

	mu       sync.Mutex
	sequence uint64
}

// New registers snippets.json on st. readOnly must be true when st was opened without the directory's lease.
func New(st *store.Store, clock platform.Clock, readOnly bool) *Store {
	return &Store{
		clock:    clock,
		readOnly: readOnly,
		doc:      store.Doc(st, snippetsFile, schemaVersion, func() snippetsDocument { return snippetsDocument{} }),
	}
}

// ReadOnly reports whether writes are refused.
func (s *Store) ReadOnly() bool {
	return s.readOnly
}

// List returns the snippets sorted by name, malformed entries left out.
func (s *Store) List() []Snippet {
	out := s.usable()
	slices.SortStableFunc(out, func(a, b Snippet) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

// Get returns the snippet with the given id.
func (s *Store) Get(id string) (Snippet, bool) {
	all := s.usable()
	i := slices.IndexFunc(all, func(sn Snippet) bool { return sn.ID == id })
	if i < 0 {
		return Snippet{}, false
	}
	return all[i], true
}

// Create adds a snippet from the form's input and returns it. It fails with a *KeywordTakenError when the keyword
// is held by another snippet.
func (s *Store) Create(in Input) (Snippet, error) {
	if s.readOnly {
		return Snippet{}, store.ErrReadOnly
	}
	clean, err := normalizeInput(in)
	if err != nil {
		return Snippet{}, err
	}
	if err := s.checkKeyword(clean.Keyword, ""); err != nil {
		return Snippet{}, err
	}
	now := s.clock.Now()
	sn := Snippet{ID: s.nextID(now), Name: clean.Name, Keyword: clean.Keyword, Text: clean.Text, UpdatedAt: now}
	if err := s.doc.Update(func(d *snippetsDocument) { d.Snippets = append(d.Snippets, sn) }); err != nil {
		return Snippet{}, err
	}
	return sn, nil
}

// Update replaces the fields of a snippet. The same validation as Create applies; the snippet keeps its own
// keyword without counting as taken.
func (s *Store) Update(id string, in Input) (Snippet, error) {
	if s.readOnly {
		return Snippet{}, store.ErrReadOnly
	}
	if _, ok := s.Get(id); !ok {
		return Snippet{}, ErrNotFound
	}
	clean, err := normalizeInput(in)
	if err != nil {
		return Snippet{}, err
	}
	if err := s.checkKeyword(clean.Keyword, id); err != nil {
		return Snippet{}, err
	}
	var updated Snippet
	err = s.doc.Update(func(d *snippetsDocument) {
		for i := range d.Snippets {
			if d.Snippets[i].ID == id {
				d.Snippets[i].Name, d.Snippets[i].Keyword, d.Snippets[i].Text = clean.Name, clean.Keyword, clean.Text
				d.Snippets[i].UpdatedAt = s.clock.Now()
				updated = d.Snippets[i]
			}
		}
	})
	return updated, err
}

// Delete removes a snippet. The caller confirms with the user first.
func (s *Store) Delete(id string) error {
	if s.readOnly {
		return store.ErrReadOnly
	}
	if _, ok := s.Get(id); !ok {
		return ErrNotFound
	}
	return s.doc.Update(func(d *snippetsDocument) {
		d.Snippets = slices.DeleteFunc(d.Snippets, func(sn Snippet) bool { return sn.ID == id })
	})
}

// Match answers a query typed at the root. Exact holds the snippets whose keyword is the query; Prefix holds the
// others whose keyword starts with it. A blank query matches nothing.
func (s *Store) Match(query string) (exact, prefix []Snippet) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	for _, sn := range s.usable() {
		switch {
		case sn.Keyword == "":
		case sn.Keyword == q:
			exact = append(exact, sn)
		case strings.HasPrefix(sn.Keyword, q):
			prefix = append(prefix, sn)
		}
	}
	return exact, prefix
}

// checkKeyword fails when another snippet than the one with id holds the keyword. An empty keyword is never taken.
func (s *Store) checkKeyword(keyword, id string) error {
	if keyword == "" {
		return nil
	}
	for _, sn := range s.usable() {
		if sn.Keyword == keyword && sn.ID != id {
			return &KeywordTakenError{Keyword: keyword, Name: sn.Name}
		}
	}
	return nil
}

// usable returns the well-formed snippets in stored order.
func (s *Store) usable() []Snippet {
	doc, _ := s.doc.Load()
	out := make([]Snippet, 0, len(doc.Snippets))
	for _, sn := range doc.Snippets {
		if validID(sn.ID) && strings.TrimSpace(sn.Text) != "" {
			out = append(out, sn)
		}
	}
	return out
}

// nextID returns a fresh snippet id.
func (s *Store) nextID(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	return newID(now, s.sequence)
}
