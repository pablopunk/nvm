package extview

import (
	"reflect"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

const chatChangedMessage = "Chat changed. Choose the message again."

// retryMessage binds a Retry row to the transcript shown when the menu was built, as a Copy row is bound: a refresh,
// a replacement or a dismissal makes the row refuse. The store checks the same transcript against the chat it keeps.
// The original chat is never changed; the store starts a new chat from the chosen answer's request, and the palette
// shows that chat once it exists.
func (s *Session) retryMessage(title, chatID string, index int) search.Action {
	displayed, revision := s.view.Chat, s.chatRevision
	var snapshot []api.ChatMessage
	if displayed != nil {
		snapshot = snapshotChatMessages(displayed.Messages)
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		if s.dismissed || displayed == nil || s.view.Chat != displayed || s.chatRevision != revision {
			h.Toast(chatChangedMessage)
			return
		}
		if !reflect.DeepEqual(snapshot, s.view.Chat.Messages) {
			h.Toast(chatChangedMessage)
			h.Refresh()
			return
		}
		newChat, err := s.deps.Chats.Retry(chatID, index, snapshot)
		if err != nil {
			h.Toast(err.Error())
		}
		if newChat != "" {
			s.openChat(h, newChat, false)
		}
	}}
}
