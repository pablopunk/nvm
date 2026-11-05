package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// BuilderPort is what the builder services read and change on the host: the builder chats, their cleanup, and the
// extensions each chat owns. The host keeps the chats; a built-in gets copies.
type BuilderPort interface {
	Chats() []api.ChatSummary
	Chat(chatID string) (api.ChatDetail, bool)
	Cleanup() (int, error)
	OwnerOf(extension string) (chatID string, found bool)
	FilesForChat(chatID string) []string
	CanWrite(extension, chatID string) bool
}

// serveBuilder answers the builder and ownership calls. Only built-in extensions may make them.
func (h *Handler) serveBuilder(call protocol.ServiceCall) (any, error) {
	if !h.manifest.IsBuiltIn() {
		return nil, unavailable("the AI builder is for built-in extensions only")
	}
	if h.deps.Builder == nil {
		return nil, unavailable("the AI builder is not available in this build")
	}
	switch call.Op {
	case protocol.OpBuilderList:
		return h.deps.Builder.Chats(), nil
	case protocol.OpBuilderGet:
		return h.builderChat(call.Args)
	case protocol.OpBuilderCleanup:
		removed, err := h.deps.Builder.Cleanup()
		return protocol.BuilderCleanupAnswer{Removed: removed}, err
	case protocol.OpOwnerOf:
		return h.ownerOf(call.Args)
	case protocol.OpFilesForChat:
		return h.filesForChat(call.Args)
	case protocol.OpCanWrite:
		return h.canWrite(call.Args)
	}
	return nil, invalid("builder has no operation %q", call.Op)
}

func (h *Handler) builderChat(raw json.RawMessage) (protocol.AIChatFound, error) {
	var args protocol.AIChatGetArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.AIChatFound{}, err
	}
	chat, found := h.deps.Builder.Chat(args.ID)
	return protocol.AIChatFound{Chat: chat, Found: found}, nil
}

func (h *Handler) ownerOf(raw json.RawMessage) (protocol.BuilderOwnerAnswer, error) {
	var args protocol.BuilderExtensionArgs
	if err := decodeArgs(raw, &args); err != nil {
		return protocol.BuilderOwnerAnswer{}, err
	}
	chatID, found := h.deps.Builder.OwnerOf(args.Extension)
	return protocol.BuilderOwnerAnswer{ChatID: chatID, Found: found}, nil
}

func (h *Handler) filesForChat(raw json.RawMessage) ([]string, error) {
	var args protocol.BuilderFilesArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	return h.deps.Builder.FilesForChat(args.ChatID), nil
}

func (h *Handler) canWrite(raw json.RawMessage) (bool, error) {
	var args protocol.BuilderCanWriteArgs
	if err := decodeArgs(raw, &args); err != nil {
		return false, err
	}
	return h.deps.Builder.CanWrite(args.Extension, args.ChatID), nil
}
