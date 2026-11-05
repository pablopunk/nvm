package app

import (
	"errors"
	"log"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// extensionChats reads the host's chats for the extensions that hold the ai capability.
type extensionChats struct {
	p *App
}

// List returns the chats, in the order the chat store keeps them.
func (c extensionChats) List() []api.ChatSummary {
	var out []api.ChatSummary
	c.p.onChatsThread(func() {
		if c.p.chats == nil {
			return
		}
		records := c.p.chats.Chats()
		out = make([]api.ChatSummary, 0, len(records))
		for _, record := range records {
			out = append(out, chatSummary(record))
		}
	})
	return out
}

// Get returns one chat with its transcript, and false when the store has no chat with the id.
func (c extensionChats) Get(chatID string) (api.ChatDetail, bool) {
	var detail api.ChatDetail
	found := false
	c.p.onChatsThread(func() {
		if c.p.chats == nil {
			return
		}
		record, ok := c.p.chats.Chat(chatID)
		if !ok {
			return
		}
		messages := make([]api.ChatMessage, 0, len(record.Messages))
		for _, message := range record.Messages {
			messages = append(messages, apiChatMessage(message))
		}
		detail, found = api.ChatDetail{ChatSummary: chatSummary(record), Messages: messages}, true
	})
	return detail, found
}

// onChatsThread runs read on the UI thread, where the chat cache lives, and waits until it has run. Extension services
// call the chat reads from their own goroutines.
func (p *App) onChatsThread(read func()) {
	done := make(chan struct{})
	p.onUI(func() {
		read()
		close(done)
	})
	<-done
}

// extensionChatPages is the chat store as the chat actions of extensions use it. The built-in shows the chats it starts.
type extensionChatPages struct {
	p *App
}

// Start creates a chat of kind, bound to extension when it is not empty, and sends prompt first when it is not empty. It
// returns the new chat's ID.
func (c extensionChatPages) Start(kind, prompt, extension string) (string, error) {
	if c.p.chats == nil {
		return "", errChatsUnavailable
	}
	chatKind, tier := startedChatKind(kind)
	record, err := c.p.chats.NewChat(chatKind, tier)
	if err != nil {
		return "", err
	}
	if extension != "" {
		if err := c.p.chats.Bind(record.ID, extension); err != nil {
			log.Println("chat: could not bind the chat to its extension:", err)
		}
		if err := c.p.chats.Retitle(record.ID, tweakChatTitle(c.p.extensionTitle(extension))); err != nil {
			log.Println("chat: could not title the chat:", err)
		}
	}
	if prompt != "" {
		c.p.onUI(func() {
			if err := c.p.chats.Send(record.ID, prompt); err != nil {
				log.Println("chat: could not send:", err)
			}
		})
	}
	return record.ID, nil
}

// tweakChatTitle names a builder chat that edits the extension titled title, as 1.x names its tweak chats.
func tweakChatTitle(title string) string {
	return "Tweak " + title
}

// startedChatKind is the store's kind and tier of a chat a chat action starts: a conversation runs on the fast tier, a
// builder chat on the smart tier.
func startedChatKind(kind string) (chat.Kind, chat.Tier) {
	if kind == "builder" {
		return chat.KindBuilder, chat.TierSmart
	}
	return chat.KindConversation, chat.TierFast
}

// Send sends text and pictures to the chat with id, as its composer does. Empty text with no pictures allows the tool
// call that waits, as Enter on an empty composer does.
func (c extensionChatPages) Send(chatID, text string, images []chat.Image) error {
	if c.p.chats == nil {
		return errChatsUnavailable
	}
	if text == "" && len(images) == 0 && c.p.chats.Awaiting(chatID) {
		return c.p.chats.Decide(chatID, true)
	}
	return c.p.chats.SendWithImages(chatID, text, images)
}

// Stop ends the reply the chat with id is streaming.
func (c extensionChatPages) Stop(chatID string) {
	if c.p.chats != nil {
		c.p.chats.Stop(chatID)
	}
}

// SetTier changes the model tier the chat with id uses for its next reply.
func (c extensionChatPages) SetTier(chatID string, tier chat.Tier) error {
	if c.p.chats == nil {
		return errChatsUnavailable
	}
	return c.p.chats.SetTier(chatID, tier)
}

// Regenerate drops the last reply of the chat with id and asks its last question again.
func (c extensionChatPages) Regenerate(chatID string) error {
	if c.p.chats == nil {
		return errChatsUnavailable
	}
	return c.p.chats.Regenerate(chatID)
}

// Decide answers the tool call the chat with id waits on: allowed or refused.
func (c extensionChatPages) Decide(chatID string, allow bool) error {
	if c.p.chats == nil {
		return errChatsUnavailable
	}
	return c.p.chats.Decide(chatID, allow)
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
		ExtensionID:    record.Extension,
		Kind:           string(record.Kind),
		Tier:           api.ModelTier(record.Tier),
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
		if message.Notice == nil {
			return api.ChatMessage{Role: "system"}
		}
		return api.ChatMessage{Role: "system", Title: message.Notice.Title, Content: message.Notice.Message}
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
