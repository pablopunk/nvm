package chat

import (
	"testing"
	"time"
)

// TestOpeningAChatRecordsNothingUntilItsFirstMessage checks an opened chat is shown and can be named, but leaves no
// record in the store or the history until a message is written.
func TestOpeningAChatRecordsNothingUntilItsFirstMessage(t *testing.T) {
	f := newFixture(t)
	chat := f.newChat()
	if _, ok := f.svc.Chat(chat.ID); !ok {
		t.Fatal("the opened chat is not shown")
	}
	if stored, _ := f.store.All(); len(stored) != 0 || len(f.svc.Chats()) != 0 {
		t.Fatalf("stored = %v, listed = %v, want nothing before the first message", stored, f.svc.Chats())
	}
	if err := f.svc.SetTier(chat.ID, TierSmart); err != nil {
		t.Fatalf("SetTier: %v", err)
	}
	if stored, _ := f.store.All(); len(stored) != 0 {
		t.Fatalf("stored = %v after a tier change, want nothing", stored)
	}

	mustSend(t, f, chat.ID, "hello")
	f.awaitTurn(chat.ID)
	stored, _ := f.store.All()
	if len(stored) != 1 || stored[0].ID != chat.ID || stored[0].Tier != TierSmart {
		t.Fatalf("stored = %+v, want the chat recorded with its first message and tier", stored)
	}
	if len(f.svc.Chats()) != 1 {
		t.Fatalf("listed = %v, want the chat once it has a message", f.svc.Chats())
	}
}

// TestDeletingAnUnusedChatForgetsIt checks removing a chat that was never recorded needs no store write and leaves
// nothing behind.
func TestDeletingAnUnusedChatForgetsIt(t *testing.T) {
	f := newFixture(t)
	chat := f.newChat()
	if err := f.svc.Delete(chat.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := f.svc.Chat(chat.ID); ok {
		t.Fatal("the removed chat is still shown")
	}
}

// TestAnUnusedChatIsForgottenWhenRetentionPasses checks an opened chat that was never used does not outlive the
// retention window, and a fresh one is kept.
func TestAnUnusedChatIsForgottenWhenRetentionPasses(t *testing.T) {
	f := newFixture(t)
	old := f.newChat()
	f.set.Clock.Advance(RetentionWindow + time.Minute)
	fresh := f.newChat()
	if _, err := f.svc.Expire(); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if _, ok := f.svc.Chat(old.ID); ok {
		t.Fatal("the unused chat past the retention window is still shown")
	}
	if _, ok := f.svc.Chat(fresh.ID); !ok {
		t.Fatal("the fresh unused chat was forgotten")
	}
}

// TestBuilderChatIsRecordedOnItsFirstMessage checks a builder chat follows the same rule: it is bound to its extension
// while it is opened, and recorded, with the binding, when its first message is written.
func TestBuilderChatIsRecordedOnItsFirstMessage(t *testing.T) {
	f := newFixture(t)
	builder, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}
	if err := f.svc.Bind(builder.ID, "user.clock"); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if stored, _ := f.store.All(); len(stored) != 0 {
		t.Fatalf("stored = %v before the first message, want nothing", stored)
	}
	mustSend(t, f, builder.ID, "a clock")
	f.awaitTurn(builder.ID)
	stored, _ := f.store.All()
	if len(stored) != 1 || stored[0].Extension != "user.clock" || stored[0].Kind != KindBuilder {
		t.Fatalf("stored = %+v, want the builder chat with its binding", stored)
	}
}
