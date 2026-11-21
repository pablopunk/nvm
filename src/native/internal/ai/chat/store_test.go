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

// recordedChat is a chat of kind with one message, last changed at at: the shape the service records.
func recordedChat(id string, kind Kind, at time.Time) Chat {
	return Chat{
		ID: id, Kind: kind, Tier: TierFast, CreatedAt: at, UpdatedAt: at,
		Messages: []Message{{Author: AuthorUser, Text: "hello", Status: StatusComplete, At: at}},
	}
}

// TestExpireRemovesOnlyTheIdleConversationsNotInUse runs retention on the store: the idle conversation goes, a
// conversation in use stays, and so does a fresh one.
func TestExpireRemovesOnlyTheIdleConversationsNotInUse(t *testing.T) {
	clock := fake.NewClock()
	st := openTestStore(t, clock)
	idle := recordedChat("idle", KindConversation, clock.Now())
	busy := recordedChat("busy", KindConversation, clock.Now())
	mustInsert(t, st, idle, busy)
	clock.Advance(RetentionWindow - time.Hour)
	fresh := recordedChat("fresh", KindConversation, clock.Now())
	mustInsert(t, st, fresh)
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

// TestEmptyChatsAreIgnoredOnLoadAndRemovedByRetention checks a chat saved with no message, as an older build could,
// is not listed, and that retention removes it at once, whatever its age and kind.
func TestEmptyChatsAreIgnoredOnLoadAndRemovedByRetention(t *testing.T) {
	clock := fake.NewClock()
	st := openTestStore(t, clock)
	empty := Chat{ID: "empty", Kind: KindConversation, Tier: TierFast, CreatedAt: clock.Now(), UpdatedAt: clock.Now()}
	emptyBuilder := Chat{ID: "empty-builder", Kind: KindBuilder, Tier: TierSmart, CreatedAt: clock.Now(), UpdatedAt: clock.Now()}
	kept := recordedChat("kept", KindConversation, clock.Now())
	mustInsert(t, st, empty, emptyBuilder, kept)

	chats, _ := st.All()
	if len(chats) != 1 || chats[0].ID != kept.ID {
		t.Fatalf("listed = %v, want only the chat with a message", chats)
	}
	removed, err := st.Expire(clock.Now(), func(string) bool { return false })
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if len(removed) != 2 || !contains(removed, empty.ID) || !contains(removed, emptyBuilder.ID) {
		t.Fatalf("removed = %v, want both empty chats", removed)
	}
}

// TestAllListsTheMostRecentlyChangedFirst checks the order the palette lists chats in.
func TestAllListsTheMostRecentlyChangedFirst(t *testing.T) {
	clock := fake.NewClock()
	st := openTestStore(t, clock)
	older := recordedChat("older", KindConversation, clock.Now())
	clock.Advance(time.Minute)
	newer := recordedChat("newer", KindConversation, clock.Now())
	mustInsert(t, st, older, newer)
	chats, _ := st.All()
	if len(chats) != 2 || chats[0].ID != newer.ID || chats[1].ID != older.ID {
		t.Fatalf("order = %v, want the newer chat first", chats)
	}
}

// mustInsert records chats in st, failing the test on a refused write.
func mustInsert(t *testing.T, st *Store, chats ...Chat) {
	t.Helper()
	for _, chat := range chats {
		if err := st.Insert(chat); err != nil {
			t.Fatalf("Insert: %v", err)
		}
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

// contains reports whether ids holds id.
func contains(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}
