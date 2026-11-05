package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// chatOperation runs a chat action on the host: the chat pages open on the palette's stack, and the chat store takes the
// sends, stops, tier changes and removals. A failed send or removal is a toast; the page it came from stays. A Continue
// that carries a message or pictures sends them in the page it runs from, so the transcript updates in place; without
// one it opens the chat.
func (s *Session) chatOperation(action api.Action, title string) search.Action {
	payload, pages := *action.Chat, s.deps.Chats
	switch action.Op {
	case api.ChatNew:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(pages.New(payload.Prompt)) }}
	case api.ChatContinue:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if payload.Message == "" && len(payload.Images) == 0 {
				h.Push(pages.Open(payload.ChatID))
				return
			}
			if err := pages.Send(payload.ChatID, payload.Message, chatImages(payload.Images)); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Refresh()
		}}
	case api.ChatStop:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			pages.Stop(payload.ChatID)
			h.Refresh()
		}}
	case api.ChatTier:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := pages.SetTier(payload.ChatID, chatTier(payload.Tier)); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Refresh()
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
