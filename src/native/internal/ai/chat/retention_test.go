package chat

import (
	"testing"
	"time"
)

// TestRetentionExpiresIdleConversationsOnly checks the window: a conversation idle past it goes, a fresher one stays,
// and a builder chat is never expired by this rule.
func TestRetentionExpiresIdleConversationsOnly(t *testing.T) {
	now := time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	chats := []Chat{
		{ID: "old", Kind: KindConversation, UpdatedAt: now.Add(-RetentionWindow - time.Minute)},
		{ID: "edge", Kind: KindConversation, UpdatedAt: now.Add(-RetentionWindow)},
		{ID: "fresh", Kind: KindConversation, UpdatedAt: now.Add(-time.Hour)},
		{ID: "builder", Kind: KindBuilder, UpdatedAt: now.Add(-48 * time.Hour)},
	}
	got := expiredIDs(chats, now)
	if len(got) != 2 || got[0] != "old" || got[1] != "edge" {
		t.Fatalf("expired = %v, want old and edge", got)
	}
}
