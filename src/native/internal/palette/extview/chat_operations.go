package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// chatOperation runs a chat action on the host: the chat pages open on the palette's stack, and the chat store takes the
// sends and the removals. A failed send or removal is a toast; the page it came from stays.
func (s *Session) chatOperation(action api.Action, title string) search.Action {
	payload, pages := *action.Chat, s.deps.Chats
	switch action.Op {
	case api.ChatNew:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(pages.New(payload.Prompt)) }}
	case api.ChatContinue:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if payload.Message != "" {
				if err := pages.Send(payload.ChatID, payload.Message); err != nil {
					h.Toast(err.Error())
					return
				}
			}
			h.Push(pages.Open(payload.ChatID))
		}}
	case api.ChatRemove:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := pages.Remove(payload.ChatID); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Refresh()
		}}
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(pages.Open(payload.ChatID)) }}
}
