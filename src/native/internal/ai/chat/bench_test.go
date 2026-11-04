package chat

import (
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// benchChats is the size the palette's budgets are measured at: 500 chats of 20 messages each (PLAN.md J29).
const (
	benchChats    = 500
	benchMessages = 20
)

// benchService returns a service over benchChats chats, and their ids.
func benchService(b *testing.B) (*Service, []string) {
	b.Helper()
	clock := fake.NewClock()
	st, err := store.Open(fake.NewFS(), clock, "/chats")
	if err != nil {
		b.Fatalf("store.Open: %v", err)
	}
	b.Cleanup(func() { _ = st.Close() })
	docs := OpenStore(st, clock)
	messages := make([]Message, 0, benchMessages)
	for i := 0; i < benchMessages; i++ {
		messages = append(messages, Message{Author: AuthorUser, Text: fmt.Sprintf("message %d with some text", i), Status: StatusComplete, At: clock.Now()})
	}
	ids := make([]string, 0, benchChats)
	for i := 0; i < benchChats; i++ {
		chat, err := docs.Create(KindConversation, TierFast)
		if err != nil {
			b.Fatalf("Create: %v", err)
		}
		if err := docs.Mutate(chat.ID, clock.Now().Add(time.Duration(i)*time.Second), func(c *Chat) { c.Messages = messages }); err != nil {
			b.Fatalf("Mutate: %v", err)
		}
		ids = append(ids, chat.ID)
	}
	svc := New(Options{Store: docs, Clock: clock, Connected: func() bool { return false }, Dispatch: func(fn func()) { fn() }})
	return svc, ids
}

// BenchmarkChatRead is what a view pays per frame to read the chat it shows. It must not grow with the number of
// chats, since the views read on every frame.
func BenchmarkChatRead(b *testing.B) {
	svc, ids := benchService(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := svc.Chat(ids[i%len(ids)]); !ok {
			b.Fatal("chat missing")
		}
	}
}

// BenchmarkStreamDelta is the cost of one piece of streamed reply text: the write the UI thread makes per delta.
func BenchmarkStreamDelta(b *testing.B) {
	svc, ids := benchService(b)
	id := ids[0]
	if err := svc.NewReplyForBench(id); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.appendText(id, "x")
	}
}

// NewReplyForBench starts a streaming reply in the chat, so a benchmark can append to it.
func (s *Service) NewReplyForBench(id string) error {
	return s.mutate(id, func(c *Chat) {
		c.Messages = append(c.Messages, s.message(AuthorAssistant, "", StatusStreaming))
	})
}
