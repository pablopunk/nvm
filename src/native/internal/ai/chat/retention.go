package chat

import "time"

// RetentionWindow is how long an idle conversation is kept. The desktop app expires conversations the same way.
const RetentionWindow = 24 * time.Hour

// removableIDs returns the ids retention removes: every chat with no message, and each conversation whose last change
// is at or before the retention cutoff. A builder chat is removed only when it is empty.
func removableIDs(chats []Chat, now time.Time) []string {
	cutoff := now.Add(-RetentionWindow)
	var ids []string
	for _, chat := range chats {
		idle := chat.Kind == KindConversation && !chat.UpdatedAt.After(cutoff)
		if idle || isEmpty(chat) {
			ids = append(ids, chat.ID)
		}
	}
	return ids
}
