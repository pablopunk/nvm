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

// All returns every chat that has a message, most recently changed first. A chat saved with no message, which an older
// build could write, is ignored here and removed by Expire.
func (s *Store) All() ([]Chat, error) {
	loaded, err := s.doc.Load()
	chats := slices.DeleteFunc(slices.Clone(loaded.Chats), isEmpty)
	slices.SortStableFunc(chats, func(a, b Chat) int { return b.UpdatedAt.Compare(a.UpdatedAt) })
	return chats, err
}

// Insert records chat as it is. The service records a chat when its first message is written, never before.
func (s *Store) Insert(chat Chat) error {
	return s.doc.Update(func(d *document) { d.Chats = append(d.Chats, chat) })
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

// Expire deletes the chats retention removes, except those keep reports as in use: every chat with no message, and the
// conversations idle past the retention window. It returns the ids it deleted.
func (s *Store) Expire(now time.Time, keep func(id string) bool) ([]string, error) {
	var removed []string
	err := s.doc.Update(func(d *document) {
		expired := removableIDs(d.Chats, now)
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

// isEmpty reports whether chat has no message yet.
func isEmpty(chat Chat) bool {
	return len(chat.Messages) == 0
}

// newChatID returns a random chat identifier.
func newChatID() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return "chat-" + hex.EncodeToString(raw[:])
}
