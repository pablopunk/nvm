package aichats

import "github.com/pablopunk/nvm/native/internal/ext/api"

// OpenChatRef is the reference of the action that shows the transcript of the chat with id. The host runs it when it
// opens a chat it created, so the built-in's view is the only one the user sees.
func OpenChatRef(chatID string) api.ActionRef {
	return api.ActionRef{ID: opOpenChat, Args: map[string]string{argChatID: chatID}}
}

// HistoryRef is the reference of the action that shows the AI Chats list.
func HistoryRef() api.ActionRef {
	return api.ActionRef{ID: CommandAIChats}
}

// LoadingChatView is the transcript of a chat the host just started, before the built-in has read it. It refreshes on a
// tick until the chat's view is ready, so the palette opens at once and fills in place.
func LoadingChatView(chatID string) api.View {
	return api.View{
		Kind: api.ViewChat, Title: "AI Chat",
		Chat:    &api.ChatSpec{Tier: api.ModelFast, Submit: chatRef(opSubmit, chatID), Stop: chatRef(opStop, chatID), OnTier: chatRef(opTier, chatID)},
		Refresh: &api.Refresh{IntervalMs: streamTickMs, Ref: chatRef(opRefreshChat, chatID)},
	}
}

// RefreshChatRef is the reference of the action that returns the transcript of the chat with id as it is now.
func RefreshChatRef(chatID string) api.ActionRef {
	return api.ActionRef{ID: opRefreshChat, Args: map[string]string{argChatID: chatID}}
}

// SubmitChatID resolves only this built-in's own submit reference.
func SubmitChatID(ref *api.ActionRef) (string, bool) {
	if ref == nil || ref.ID != opSubmit || ref.Args[argChatID] == "" {
		return "", false
	}
	return ref.Args[argChatID], true
}
