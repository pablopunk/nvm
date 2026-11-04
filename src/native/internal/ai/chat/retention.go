package chat

import "time"

// RetentionWindow is how long an idle conversation is kept. The desktop app expires conversations the same way.
const RetentionWindow = 24 * time.Hour

// expiredIDs returns the ids of the conversations whose last change is at or before the retention cutoff.
func expiredIDs(chats []Chat, now time.Time) []string {
	cutoff := now.Add(-RetentionWindow)
	var ids []string
	for _, chat := range chats {
		if chat.Kind != KindConversation {
			continue
		}
		if !chat.UpdatedAt.After(cutoff) {
			ids = append(ids, chat.ID)
		}
	}
	return ids
}
