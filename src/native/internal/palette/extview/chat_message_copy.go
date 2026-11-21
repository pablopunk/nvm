package extview

import (
	"reflect"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

type chatCopyState struct {
	view     *api.ChatSpec
	revision uint64
	identity *byte
	copy     func(search.Host, int)
}

// messageCopier binds the action to the displayed transcript. A refresh invalidates an open action menu.
func (s *Session) messageCopier() func(search.Host, int) {
	if s.deps.CopyText == nil || s.view.Chat == nil {
		return nil
	}
	displayed := s.view.Chat
	if s.chatCopies != nil && s.chatCopies.view == displayed && s.chatCopies.revision == s.chatRevision {
		return s.chatCopies.copy
	}
	revision := s.chatRevision
	state := &chatCopyState{view: displayed, revision: revision, identity: new(byte)}
	snapshot := snapshotChatMessages(displayed.Messages)
	copyMessage := func(h search.Host, index int) {
		if s.dismissed || s.view.Chat == nil || s.view.Chat != displayed || s.chatRevision != revision {
			h.Toast(chatChangedMessage)
			return
		}
		if !reflect.DeepEqual(snapshot, s.view.Chat.Messages) {
			if s.chatCopies == state {
				s.chatCopies = nil
			}
			h.Toast(chatChangedMessage)
			h.Refresh()
			return
		}
		if index < 0 || index >= len(snapshot) {
			h.Toast(chatChangedMessage)
			return
		}
		message := snapshot[index]
		if message.Tool == nil && (message.Role == "user" || message.Role == "assistant") && message.Content != "" {
			s.deps.CopyText(h, message.Content)
		}
	}
	state.copy = copyMessage
	s.chatCopies = state
	return copyMessage
}

// snapshotChatMessages retains the values of a transcript. Strings are immutable: it copies only slice and tool
// metadata, never image bytes.
func snapshotChatMessages(messages []api.ChatMessage) []api.ChatMessage {
	snapshot := slices.Clone(messages)
	for i, message := range snapshot {
		snapshot[i].Images = slices.Clone(message.Images)
		if message.Tool != nil {
			tool := *message.Tool
			snapshot[i].Tool = &tool
		}
	}
	return snapshot
}
