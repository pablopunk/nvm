package extview

import (
	"bytes"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/search"
)

// messageCopier binds the action to the displayed transcript. A refresh invalidates an open action menu.
func (s *Session) messageCopier() func(search.Host, int) {
	if s.deps.CopyText == nil || s.view.Chat == nil {
		return nil
	}
	displayed := s.view.Chat
	snapshot, err := json.Marshal(displayed.Messages)
	if err != nil {
		return nil
	}
	return func(h search.Host, index int) {
		if s.view.Chat == nil || s.view.Chat != displayed {
			h.Toast("Chat changed. Choose the message again.")
			return
		}
		current, err := json.Marshal(s.view.Chat.Messages)
		if err != nil || !bytes.Equal(snapshot, current) || index < 0 || index >= len(s.view.Chat.Messages) {
			h.Toast("Chat changed. Choose the message again.")
			return
		}
		message := s.view.Chat.Messages[index]
		if message.Tool == nil && (message.Role == "user" || message.Role == "assistant") && message.Content != "" {
			s.deps.CopyText(h, message.Content)
		}
	}
}
