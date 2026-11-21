package app

import (
	"slices"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Retry starts a new chat from the request the answer at index replied to, and returns it. The chat the user chose from
// and its later messages stay as they are. shown is the transcript the user chose from: when the chat has changed since,
// the store refuses with chat.ErrStaleFork instead of retrying a different answer. A builder chat is refused with
// chat.ErrBuilderFork, as an extension has one builder chat.
func (c extensionChatPages) Retry(chatID string, index int, shown []api.ChatMessage) (string, error) {
	if c.p.chats == nil {
		return "", errChatsUnavailable
	}
	source, ok := c.p.chats.Chat(chatID)
	if !ok {
		return "", chat.ErrNotFound
	}
	if !sameTranscript(shown, source.Messages) {
		return "", chat.ErrStaleFork
	}
	fork, err := c.p.chats.ForkReply(source, index)
	return fork.ID, err
}

// sameTranscript reports whether the messages an extension showed are the ones the chat holds now, as it reads them.
func sameTranscript(shown []api.ChatMessage, held []chat.Message) bool {
	if len(shown) != len(held) {
		return false
	}
	for i, message := range held {
		current := apiChatMessage(message)
		seen := shown[i]
		if current.Role != seen.Role || current.Title != seen.Title || current.Content != seen.Content ||
			current.Thinking != seen.Thinking || current.Streaming != seen.Streaming ||
			!slices.EqualFunc(current.Images, seen.Images, sameChatImage) || !sameChatTool(current.Tool, seen.Tool) {
			return false
		}
	}
	return true
}

func sameChatImage(a, b api.ChatImage) bool { return a == b }

func sameChatTool(a, b *api.ChatTool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
