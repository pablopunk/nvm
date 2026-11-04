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

// Author says who wrote a message. Notices are in the transcript so that a refusal stays visible after a restart, and a
// tool entry is one call the assistant made, shown as an activity row.
type Author string

// The message authors.
const (
	AuthorUser      Author = "user"
	AuthorAssistant Author = "assistant"
	AuthorNotice    Author = "notice"
	AuthorTool      Author = "tool"
)

// Status is where a message stands. Only a streaming message accepts text. Tool entries use the running, awaiting
// and end statuses.
type Status string

// The message statuses.
const (
	StatusStreaming Status = "streaming"
	StatusComplete  Status = "complete"
	StatusStopped   Status = "stopped"
	StatusRunning   Status = "running"
	StatusAwaiting  Status = "awaiting"
	StatusFailed    Status = "failed"
	StatusRefused   Status = "refused"
	StatusCancelled Status = "cancelled"
)

// ToolRun describes one tool call in the transcript: what it is for, its arguments in short form, and the line that
// says how it ended. The text of its message is the result the model read.
type ToolRun struct {
	CallID  string `json:"callId"`
	Name    string `json:"name"`
	Label   string `json:"label"`
	Input   string `json:"input,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

// Message is one entry of a transcript. Notice is set for AuthorNotice messages only. Calls are the tool calls an
// assistant message ended with; Tool describes a tool entry, which is one call shown as an activity row.
type Message struct {
	Author Author             `json:"author"`
	Text   string             `json:"text"`
	Status Status             `json:"status"`
	Notice *Notice            `json:"notice,omitempty"`
	Calls  []backend.ToolCall `json:"calls,omitempty"`
	Tool   *ToolRun           `json:"tool,omitempty"`
	At     time.Time          `json:"at"`
}

// Chat is one conversation: its kind and tier, its transcript and when it was last changed. A builder chat names the
// extension it wrote or edits in Extension; that binding is the chat's ownership of the extension.
type Chat struct {
	ID        string    `json:"id"`
	Kind      Kind      `json:"kind"`
	Title     string    `json:"title"`
	Tier      Tier      `json:"tier"`
	Extension string    `json:"extension,omitempty"`
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

// AwaitingAnswer reports whether a tool call of the chat waits for the user's answer, as the transcript shows it.
func AwaitingAnswer(chat Chat) bool {
	for _, message := range chat.Messages {
		if message.Author == AuthorTool && message.Status == StatusAwaiting {
			return true
		}
	}
	return false
}
