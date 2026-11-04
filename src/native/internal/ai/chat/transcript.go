package chat

import (
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// Limits on what one request carries: the most recent turns, and the size of each message's text.
const (
	MaxHistoryMessages = 20
	maxMessageRunes    = 12000
)

// ConversationSystemPrompt is the instruction every conversation starts with.
const ConversationSystemPrompt = "You are Nevermind AI, a helpful conversational assistant. Answer the user directly and concisely. You cannot create, modify, validate, install, or remove Nevermind extensions."

// Request builds the chat messages for the next turn: the system prompt, then the most recent settled turns.
// It skips notices and streaming replies; a stopped reply stays, so a resumed chat sees what the user saw. The caller makes
// sure the chat ends with the user message it is answering.
func Request(system string, chat Chat) []backend.Message {
	messages := []backend.Message{{Role: backend.RoleSystem, Content: backend.TextContent(system)}}
	var turns []backend.Message
	for _, message := range chat.Messages {
		if turn, ok := turnOf(message); ok {
			turns = append(turns, turn)
		}
	}
	if len(turns) > MaxHistoryMessages {
		turns = turns[len(turns)-MaxHistoryMessages:]
	}
	return append(messages, turns...)
}

// turnOf converts a message into a chat message, or reports that it does not belong in the history.
func turnOf(message Message) (backend.Message, bool) {
	switch {
	case message.Author == AuthorUser && message.Text != "":
		return backend.Message{Role: backend.RoleUser, Content: backend.TextContent(truncateRunes(message.Text))}, true
	case message.Author == AuthorAssistant && message.Status != StatusStreaming && message.Text != "":
		return backend.Message{Role: backend.RoleAssistant, Content: backend.TextContent(truncateRunes(message.Text))}, true
	}
	return backend.Message{}, false
}

// truncateRunes keeps the first maxMessageRunes runes of text.
func truncateRunes(text string) string {
	if utf8.RuneCountInString(text) <= maxMessageRunes {
		return text
	}
	return string([]rune(text)[:maxMessageRunes])
}

// RequestSize is the number of characters a request carries, which the backend prices.
func RequestSize(messages []backend.Message) int {
	size := 0
	for _, message := range messages {
		size += len(message.Content)
	}
	return size
}
