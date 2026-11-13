package chat

import "testing"

// TestDeletingARecordedChatReportsIt checks the host hears about a deleted chat, so it can remove its extension, and
// does not hear about a chat that was only opened.
func TestDeletingARecordedChatReportsIt(t *testing.T) {
	f := newFixture(t)
	var removed []Chat
	f.svc.opts.OnRemoved = func(chat Chat) { removed = append(removed, chat) }

	unused, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}
	if err := f.svc.Bind(unused.ID, "user.unused"); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if err := f.svc.Delete(unused.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("an unused chat was reported as removed: %+v", removed)
	}

	builder, err := f.svc.NewChat(KindBuilder, TierSmart)
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}
	if err := f.svc.Bind(builder.ID, "user.clock"); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	mustSend(t, f, builder.ID, "a clock")
	f.awaitTurn(builder.ID)
	if err := f.svc.Delete(builder.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(removed) != 1 || removed[0].Extension != "user.clock" {
		t.Fatalf("removed = %+v, want the builder chat with its extension", removed)
	}
}
