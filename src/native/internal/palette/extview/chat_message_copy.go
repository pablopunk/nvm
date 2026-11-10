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
	// Strings are immutable: retain their values, copying only slice and tool metadata, never image bytes.
	snapshot := slices.Clone(displayed.Messages)
	for i, message := range snapshot {
		snapshot[i].Images = slices.Clone(message.Images)
		if message.Tool != nil {
			tool := *message.Tool
			snapshot[i].Tool = &tool
		}
	}
	copyMessage := func(h search.Host, index int) {
		if s.dismissed || s.view.Chat == nil || s.view.Chat != displayed || s.chatRevision != revision {
			h.Toast("Chat changed. Choose the message again.")
			return
		}
		if !reflect.DeepEqual(snapshot, s.view.Chat.Messages) || index < 0 || index >= len(snapshot) {
			h.Toast("Chat changed. Choose the message again.")
			return
		}
		message := snapshot[index]
		if message.Tool == nil && (message.Role == "user" || message.Role == "assistant") && message.Content != "" {
			s.deps.CopyText(h, message.Content)
		}
	}
	s.chatCopies = &chatCopyState{view: displayed, revision: revision, identity: new(byte), copy: copyMessage}
	return copyMessage
}
