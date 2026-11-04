package chat

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/store"
)

// chatsFile is the document name and schemaVersion the chats start at (store/MIGRATING.md). The prototype's
// chats.json wrote no version, so the store quarantines it instead of reading it as chats.
const (
	chatsFile     = "chats.json"
	schemaVersion = 1
)

// document is the chats.json body.
type document struct {
	Chats []Chat `json:"chats"`
}

// Store holds the chats in one store document. It is safe for concurrent use; the document's lock serialises
// every change.
type Store struct {
	doc   *store.Collection[document]
	clock platform.Clock
}

// OpenStore registers chats.json on st. The caller opens and closes st.
func OpenStore(st *store.Store, clock platform.Clock) *Store {
	return &Store{
		doc:   store.Doc(st, chatsFile, schemaVersion, func() document { return document{} }),
		clock: clock,
	}
}

// All returns every chat, most recently changed first.
func (s *Store) All() ([]Chat, error) {
	loaded, err := s.doc.Load()
	chats := slices.Clone(loaded.Chats)
	slices.SortStableFunc(chats, func(a, b Chat) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return chats, err
}

// Create adds an empty chat of kind and tier and returns it.
func (s *Store) Create(kind Kind, tier Tier) (Chat, error) {
	now := s.clock.Now()
	chat := Chat{ID: newChatID(), Kind: kind, Tier: tier, CreatedAt: now, UpdatedAt: now}
	if err := s.doc.Update(func(d *document) { d.Chats = append(d.Chats, chat) }); err != nil {
		return Chat{}, err
	}
	return chat, nil
}

// Mutate applies change to the chat with id and stamps it as changed at now.
func (s *Store) Mutate(id string, now time.Time, change func(*Chat)) error {
	found := false
	err := s.doc.Update(func(d *document) {
		for i := range d.Chats {
			if d.Chats[i].ID != id {
				continue
			}
			change(&d.Chats[i])
			d.Chats[i].UpdatedAt = now
			found = true
			return
		}
	})
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}

// Remove deletes the chat with id. Removing a chat that is gone is not an error.
func (s *Store) Remove(id string) error {
	return s.doc.Update(func(d *document) {
		d.Chats = slices.DeleteFunc(d.Chats, func(chat Chat) bool { return chat.ID == id })
	})
}

// Expire deletes the conversations that have been idle past the retention window, except those keep reports
// as in use. It returns the ids it deleted. Builder chats are not expired.
func (s *Store) Expire(now time.Time, keep func(id string) bool) ([]string, error) {
	var removed []string
	err := s.doc.Update(func(d *document) {
		expired := expiredIDs(d.Chats, now)
		d.Chats = slices.DeleteFunc(d.Chats, func(chat Chat) bool {
			if !slices.Contains(expired, chat.ID) || keep(chat.ID) {
				return false
			}
			removed = append(removed, chat.ID)
			return true
		})
	})
	return removed, err
}

// newChatID returns a random chat identifier.
func newChatID() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return "chat-" + hex.EncodeToString(raw[:])
}
