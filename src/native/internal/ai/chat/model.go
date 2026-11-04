package chat

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// Tier is the model tier a chat uses. The backend maps it to the account's active route.
type Tier string

// The tiers. Fast is the default for a conversation; smart is the default for the extension builder.
const (
	TierFast  Tier = backend.RoleFast
	TierSmart Tier = backend.RoleSmart
)

// Kind separates the user's conversations from the extension builder's chats.
type Kind string

// The chat kinds.
const (
	KindConversation Kind = "conversation"
	KindBuilder      Kind = "builder"
)

// Author says who wrote a message. Notices are in the transcript so that a refusal stays visible after a restart.
type Author string

// The message authors.
const (
	AuthorUser      Author = "user"
	AuthorAssistant Author = "assistant"
	AuthorNotice    Author = "notice"
)

// Status is where an assistant message stands. Only a streaming message accepts text.
type Status string

// The message statuses.
const (
	StatusStreaming Status = "streaming"
	StatusComplete  Status = "complete"
	StatusStopped   Status = "stopped"
)

// Message is one entry of a transcript. Notice is set for AuthorNotice messages only.
type Message struct {
	Author Author    `json:"author"`
	Text   string    `json:"text"`
	Status Status    `json:"status"`
	Notice *Notice   `json:"notice,omitempty"`
	At     time.Time `json:"at"`
}

// Chat is one conversation: its kind and tier, its transcript and when it was last changed.
type Chat struct {
	ID        string    `json:"id"`
	Kind      Kind      `json:"kind"`
	Title     string    `json:"title"`
	Tier      Tier      `json:"tier"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// titleRunes bounds a title, the first line of the first message.
const titleRunes = 60

// TitleFor returns the title a chat takes from its first message: the first line, cut to titleRunes.
func TitleFor(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	line = strings.Join(strings.Fields(line), " ")
	if utf8.RuneCountInString(line) <= titleRunes {
		return line
	}
	return string([]rune(line)[:titleRunes-1]) + "…"
}

// LastReply returns the text of the most recent assistant message, and whether there is one.
func LastReply(chat Chat) (string, bool) {
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		if chat.Messages[i].Author == AuthorAssistant {
			return chat.Messages[i].Text, true
		}
	}
	return "", false
}

// Busy reports whether the chat has a message that is still streaming.
func Busy(chat Chat) bool {
	for _, message := range chat.Messages {
		if message.Status == StatusStreaming {
			return true
		}
	}
	return false
}
