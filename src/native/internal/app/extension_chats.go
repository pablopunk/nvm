package app

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/aichat"
)

// extensionChats reads the host's chats for the extensions that hold the ai capability.
type extensionChats struct {
	p *App
}

// List returns the chats, in the order the chat store keeps them.
func (c extensionChats) List() []api.ChatSummary {
	if c.p.chats == nil {
		return nil
	}
	records := c.p.chats.Chats()
	out := make([]api.ChatSummary, 0, len(records))
	for _, record := range records {
		out = append(out, chatSummary(record))
	}
	return out
}

// Get returns one chat with its transcript, and false when the store has no chat with the id.
func (c extensionChats) Get(chatID string) (api.ChatDetail, bool) {
	if c.p.chats == nil {
		return api.ChatDetail{}, false
	}
	record, ok := c.p.chats.Chat(chatID)
	if !ok {
		return api.ChatDetail{}, false
	}
	messages := make([]api.ChatMessage, 0, len(record.Messages))
	for _, message := range record.Messages {
		messages = append(messages, apiChatMessage(message))
	}
	return api.ChatDetail{ChatSummary: chatSummary(record), Messages: messages}, true
}

// extensionChatPages runs the chat actions of extensions on the palette's stack and the chat store.
type extensionChatPages struct {
	p *App
}

// Open returns the page of the chat with id.
func (c extensionChatPages) Open(chatID string) *palette.View {
	return aichat.ChatView(c.p.chatDeps(), chatID)
}

// New starts a conversation with prompt as its first message.
func (c extensionChatPages) New(prompt string) *palette.View {
	return c.p.newChatView(prompt)
}

// Send sends text to the chat with id, as its composer does.
func (c extensionChatPages) Send(chatID, text string) error {
	if c.p.chats == nil {
		return errChatsUnavailable
	}
	return c.p.chats.Send(chatID, text)
}

// Remove deletes the chat with id.
func (c extensionChatPages) Remove(chatID string) error {
	if c.p.chats == nil {
		return errChatsUnavailable
	}
	return c.p.chats.Delete(chatID)
}

// chatSummary is the extension's view of a chat's header.
func chatSummary(record chat.Chat) api.ChatSummary {
	return api.ChatSummary{
		ID:             record.ID,
		Kind:           string(record.Kind),
		Title:          record.Title,
		CreatedAtMs:    record.CreatedAt.UnixMilli(),
		UpdatedAtMs:    record.UpdatedAt.UnixMilli(),
		ExtensionFiles: []string{},
	}
}

// errChatsUnavailable answers a chat action when the chat store did not open.
var errChatsUnavailable = errors.New("AI chats are unavailable")

// apiChatMessage is one transcript entry as an extension reads it: its role, text, reasoning, pictures and tool row.
func apiChatMessage(message chat.Message) api.ChatMessage {
	switch message.Author {
	case chat.AuthorUser:
		return api.ChatMessage{Role: "user", Content: message.Text, Images: apiChatImages(message.Images)}
	case chat.AuthorTool:
		return api.ChatMessage{Role: "assistant", Tool: &api.ChatTool{Name: toolName(message), Status: toolStatusWord(message.Status), Result: toolOutcome(message)}}
	case chat.AuthorNotice:
		content := ""
		if message.Notice != nil {
			content = message.Notice.Message
		}
		return api.ChatMessage{Role: "system", Content: content}
	}
	return api.ChatMessage{Role: "assistant", Content: message.Text, Thinking: message.Reasoning, Streaming: message.Status == chat.StatusStreaming}
}

// apiChatImages maps the pictures of a user turn to data URLs.
func apiChatImages(images []chat.Image) []api.ChatImage {
	out := make([]api.ChatImage, 0, len(images))
	for _, image := range images {
		out = append(out, api.ChatImage{Title: image.Name, DataURL: "data:" + image.MimeType + ";base64," + image.Data})
	}
	return out
}

// toolName is the name of a tool row's tool, or empty for a row without one.
func toolName(message chat.Message) string {
	if message.Tool == nil {
		return ""
	}
	return message.Tool.Name
}

// toolOutcome is the result text of a tool row, or empty for a row without one.
func toolOutcome(message chat.Message) string {
	if message.Tool == nil {
		return ""
	}
	return message.Tool.Outcome
}

// toolStatusWord is the status word a tool row carries in the API: running, waiting, failed or done.
func toolStatusWord(status chat.Status) string {
	switch status {
	case chat.StatusRunning:
		return "running"
	case chat.StatusAwaiting:
		return "waiting"
	case chat.StatusFailed:
		return "failed"
	}
	return "done"
}
