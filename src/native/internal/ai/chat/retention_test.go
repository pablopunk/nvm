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
		recordedChat("old", KindConversation, now.Add(-RetentionWindow-time.Minute)),
		recordedChat("edge", KindConversation, now.Add(-RetentionWindow)),
		recordedChat("fresh", KindConversation, now.Add(-time.Hour)),
		recordedChat("builder", KindBuilder, now.Add(-48*time.Hour)),
	}
	got := removableIDs(chats, now)
	if len(got) != 2 || got[0] != "old" || got[1] != "edge" {
		t.Fatalf("expired = %v, want old and edge", got)
	}
}

// TestRetentionAlsoRemovesEmptyChatsOfEveryKind checks a chat with no message goes whatever its age or kind, so an
// empty builder chat an older build saved does not stay in the history.
func TestRetentionAlsoRemovesEmptyChatsOfEveryKind(t *testing.T) {
	now := time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	chats := []Chat{
		{ID: "empty-builder", Kind: KindBuilder, UpdatedAt: now},
		{ID: "empty-conversation", Kind: KindConversation, UpdatedAt: now},
		recordedChat("fresh", KindConversation, now),
	}
	got := removableIDs(chats, now)
	if len(got) != 2 || got[0] != "empty-builder" || got[1] != "empty-conversation" {
		t.Fatalf("removed = %v, want the two empty chats", got)
	}
}
