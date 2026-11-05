package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/builtins/aichats"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Kinds of chat a chat action starts. They are the kinds the chat store keeps.
const (
	kindConversation = "conversation"
	kindBuilder      = "builder"
)

// chatOperation runs a chat action on the host: the chat store takes the sends, stops, tier changes, removals and new
// chats, and the built-in that owns the chat views shows the chat a start or an open names. A failed send or removal is
// a toast; the page it came from stays.
func (s *Session) chatOperation(action api.Action, title string) search.Action {
	payload := *action.Chat
	switch action.Op {
	case api.ChatNew:
		return s.startChat(title, kindConversation, payload.Prompt, "", payload.Replace)
	case api.ChatContinue:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if payload.Message == "" && len(payload.Images) == 0 {
				s.openChat(h, payload.ChatID, false)
				return
			}
			if err := s.deps.Chats.Send(payload.ChatID, payload.Message, chatImages(payload.Images)); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Refresh()
		}}
	case api.ChatStop:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			s.deps.Chats.Stop(payload.ChatID)
			h.Refresh()
		}}
	case api.ChatTier:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if err := s.deps.Chats.SetTier(payload.ChatID, chatTier(payload.Tier)); err != nil {
				h.Toast(err.Error())
				return
			}
			h.Refresh()
		}}
	case api.ChatRemove:
		return s.removeChat(title, payload.ChatID)
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.openChat(h, payload.ChatID, false) }}
}

// builderOperation runs an AI builder action of a built-in extension: a builder chat starts or opens, the AI Chats list
// shows, and a removal is a toast when the store refuses it.
func (s *Session) builderOperation(action api.Action, title string) search.Action {
	payload := *action.Builder
	switch action.Op {
	case api.BuilderStart:
		return s.startChat(title, kindBuilder, payload.Prompt, "", false)
	case api.BuilderTweak:
		return s.startChat(title, kindBuilder, "", payload.ExtensionID, false)
	case api.BuilderList:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.runBuiltIn(h, aichats.HistoryRef(), false) }}
	case api.BuilderRemove:
		return s.removeChat(title, payload.ChatID)
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.openChat(h, payload.ChatID, false) }}
}

// startChat starts a chat of kind and shows it. The new chat replaces the page it runs from when replace is set.
func (s *Session) startChat(title, kind, prompt, extension string, replace bool) search.Action {
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		id, err := s.deps.Chats.Start(kind, prompt, extension)
		if err != nil {
			h.Toast(err.Error())
			return
		}
		s.openChat(h, id, replace)
	}}
}

// removeChat removes the chat with id; a refusal is a toast, and success refreshes the page it runs from.
func (s *Session) removeChat(title, id string) search.Action {
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		if err := s.deps.Chats.Remove(id); err != nil {
			h.Toast(err.Error())
			return
		}
		h.Refresh()
	}}
}

// openChat shows the transcript of the chat with id, as the built-in draws it.
func (s *Session) openChat(h search.Host, id string, replace bool) {
	s.runBuiltIn(h, aichats.OpenChatRef(id), replace)
}

// runBuiltIn runs one action of the built-in extension and shows the view it returns. The run is asynchronous, so a
// built-in that reads the chat store never waits on the UI thread it runs on. The view replaces the page it runs from
// when replace is set.
func (s *Session) runBuiltIn(h search.Host, ref api.ActionRef, replace bool) {
	if s.deps.Invoke == nil {
		h.Toast(unavailableMessage)
		return
	}
	s.deps.Invoke(aichats.ID, api.Invocation{Ref: ref}, func(result api.Result, failure *api.Failure) {
		if replace {
			result.Navigation = api.NavReplace
		}
		s.finish(h, aichats.ID, "", result, failure)
	})
}
