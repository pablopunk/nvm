package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteChats reads the host's chats. The reads go to the host; the actions only describe what the user picks, and the
// palette runs them.
type remoteChats struct {
	link *hostLink
}

func (c remoteChats) List() []api.ChatSummary {
	var chats []api.ChatSummary
	_ = c.link.Call(protocol.ServiceAI, protocol.OpChatList, nil, &chats)
	return chats
}

func (c remoteChats) Get(chatID string) (api.ChatDetail, bool) {
	var found protocol.AIChatFound
	if err := c.link.Call(protocol.ServiceAI, protocol.OpChatGet, protocol.AIChatGetArgs{ID: chatID}, &found); err != nil {
		return api.ChatDetail{}, false
	}
	return found.Chat, found.Found
}

func (c remoteChats) New(prompt string) api.Action {
	return chatAction("New Chat", api.ChatNew, api.ChatAction{Prompt: prompt})
}

func (c remoteChats) Open(chatID string) api.Action {
	return chatAction("Open Chat", api.ChatOpen, api.ChatAction{ChatID: chatID})
}

func (c remoteChats) Continue(chatID, message string) api.Action {
	return chatAction("Continue Chat", api.ChatContinue, api.ChatAction{ChatID: chatID, Message: message})
}

func (c remoteChats) Stop(chatID string) api.Action {
	return chatAction("Stop", api.ChatStop, api.ChatAction{ChatID: chatID})
}

func (c remoteChats) Tier(chatID string, tier api.ModelTier) api.Action {
	return chatAction("Change Tier", api.ChatTier, api.ChatAction{ChatID: chatID, Tier: tier})
}

func (c remoteChats) Remove(chatID string) api.Action {
	action := chatAction("Remove Chat", api.ChatRemove, api.ChatAction{ChatID: chatID})
	action.Confirm = &api.Confirmation{Title: "Remove this chat?", ConfirmLabel: "Remove", Destructive: true}
	return action
}

// chatAction is a declarative chat action with its payload.
func chatAction(title, op string, payload api.ChatAction) api.Action {
	return api.Action{Kind: api.ActionChat, Op: op, Title: title, Chat: &payload}
}
