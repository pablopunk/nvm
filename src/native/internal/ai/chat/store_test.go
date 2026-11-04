package chat

import (
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// openTestStore opens a chat store on a fake file system with a fake clock.
func openTestStore(t *testing.T, clock *fake.Clock) *Store {
	t.Helper()
	st, err := store.Open(fake.NewFS(), clock, "/chats")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return OpenStore(st, clock)
}

// TestExpireRemovesOnlyTheIdleConversationsNotInUse runs retention on the store: the idle conversation goes, a
// conversation in use stays, and so does a fresh one.
func TestExpireRemovesOnlyTheIdleConversationsNotInUse(t *testing.T) {
	clock := fake.NewClock()
	st := openTestStore(t, clock)
	idle, _ := st.Create(KindConversation, TierFast)
	busy, _ := st.Create(KindConversation, TierFast)
	clock.Advance(RetentionWindow - time.Hour)
	fresh, _ := st.Create(KindConversation, TierFast)
	clock.Advance(2 * time.Hour)

	removed, err := st.Expire(clock.Now(), func(id string) bool { return id == busy.ID })
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if len(removed) != 1 || removed[0] != idle.ID {
		t.Fatalf("removed = %v, want only %s", removed, idle.ID)
	}
	chats, _ := st.All()
	if len(chats) != 2 || !hasChat(chats, busy.ID) || !hasChat(chats, fresh.ID) {
		t.Fatalf("kept = %v, want the busy and the fresh chat", chats)
	}
}

// TestAllListsTheMostRecentlyChangedFirst checks the order the palette lists chats in.
func TestAllListsTheMostRecentlyChangedFirst(t *testing.T) {
	clock := fake.NewClock()
	st := openTestStore(t, clock)
	older, _ := st.Create(KindConversation, TierFast)
	clock.Advance(time.Minute)
	newer, _ := st.Create(KindConversation, TierFast)
	chats, _ := st.All()
	if len(chats) != 2 || chats[0].ID != newer.ID || chats[1].ID != older.ID {
		t.Fatalf("order = %v, want the newer chat first", chats)
	}
}

// hasChat reports whether chats holds id.
func hasChat(chats []Chat, id string) bool {
	for _, chat := range chats {
		if chat.ID == id {
			return true
		}
	}
	return false
}
