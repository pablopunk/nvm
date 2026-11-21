package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// ChatReader is what the AI service reads of the host's chats. The host keeps the transcripts; an extension gets copies.
type ChatReader interface {
	List() []api.ChatSummary
	Get(chatID string) (api.ChatDetail, bool)
}

// listChats answers chat-list with the host's chats, newest first as the host lists them.
func (h *Handler) listChats() ([]api.ChatSummary, error) {
	if h.deps.Chats == nil {
		return nil, unavailable("chats are not available in this build")
	}
	return h.deps.Chats.List(), nil
}

// getChat answers chat-get with one chat and whether the host has it.
func (h *Handler) getChat(raw json.RawMessage) (protocol.AIChatFound, error) {
	var args protocol.AIChatGetArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.AIChatFound{}, err
	}
	if h.deps.Chats == nil {
		return protocol.AIChatFound{}, unavailable("chats are not available in this build")
	}
	chat, found := h.deps.Chats.Get(args.ID)
	return protocol.AIChatFound{Chat: chat, Found: found}, nil
}
