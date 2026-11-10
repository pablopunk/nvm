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

// chatOperation runs a chat action on the host: the chat store takes the sends, stops, tier changes, removals, new chats
// and answers to tool calls, and the built-in that owns the chat views shows the chat that results. A failed store call
// is a toast; the transcript it came from stays and refreshes in place.
func (s *Session) chatOperation(action api.Action, title string) search.Action {
	payload := *action.Chat
	switch action.Op {
	case api.ChatNew:
		return s.startChat(title, kindConversation, payload.Prompt, "", payload.Replace)
	case api.ChatContinue:
		return s.chatStep(title, payload.ChatID, func() error {
			return s.deps.Chats.Send(payload.ChatID, payload.Message, chatImages(payload.Images))
		})
	case api.ChatAgain:
		return s.chatStep(title, payload.ChatID, func() error { return s.deps.Chats.Regenerate(payload.ChatID) })
	case api.ChatAllow:
		return s.chatStep(title, payload.ChatID, func() error { return s.deps.Chats.Decide(payload.ChatID, true) })
	case api.ChatRefuse:
		return s.chatStep(title, payload.ChatID, func() error { return s.deps.Chats.Decide(payload.ChatID, false) })
	case api.ChatStop:
		return s.chatStep(title, payload.ChatID, func() error {
			s.deps.Chats.Stop(payload.ChatID)
			return nil
		})
	case api.ChatTier:
		return s.chatStep(title, payload.ChatID, func() error { return s.deps.Chats.SetTier(payload.ChatID, chatTier(payload.Tier)) })
	case api.ChatRemove:
		return s.removeChat(title, payload.ChatID)
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.openChat(h, payload.ChatID, false) }}
}

// chatStep runs one store call on a chat and refreshes its transcript: a failure is a toast and the transcript stays as it was.
func (s *Session) chatStep(title, id string, step func() error) search.Action {
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		if err := step(); err != nil {
			h.Toast(err.Error())
			return
		}
		s.refreshChat(h, id)
	}}
}

// refreshChat asks the built-in for the transcript of the chat with id as it is now, and shows it in place of the one
// the palette holds, so the messages, the tier and the actions follow the store.
func (s *Session) refreshChat(h search.Host, id string) {
	s.deps.Invoke(aichats.ID, api.Invocation{Ref: aichats.RefreshChatRef(id)}, func(result api.Result, failure *api.Failure) {
		if failure == nil && result.View != nil && result.View.Kind == api.ViewChat {
			s.chatRevision++
			s.view = *result.View
		}
		h.Refresh()
	})
}

// builderOperation runs an AI builder action of a built-in extension: a builder chat starts or opens, the AI Chats list
// shows, the review of an extension opens, and a removal is a toast when the store refuses it.
func (s *Session) builderOperation(action api.Action, title string) search.Action {
	payload := *action.Builder
	switch action.Op {
	case api.BuilderStart:
		return s.startChat(title, kindBuilder, payload.Prompt, "", false)
	case api.BuilderTweak:
		return s.startChat(title, kindBuilder, "", payload.ExtensionID, false)
	case api.BuilderList:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.runBuiltIn(h, aichats.HistoryRef(), false) }}
	case api.BuilderReview:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.reviewExtension(h, payload.ExtensionID) }}
	case api.BuilderRemove:
		return s.removeChat(title, payload.ChatID)
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.openChat(h, payload.ChatID, false) }}
}

// reviewExtension pushes the review of the extension, or says it is gone when the host has no review for it.
func (s *Session) reviewExtension(h search.Host, extension string) {
	if s.deps.Review == nil {
		h.Toast(unavailableMessage)
		return
	}
	view := s.deps.Review(extension)
	if view == nil {
		h.Toast("This extension is not installed any more.")
		return
	}
	h.Push(view)
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
		s.itemsRevision++
		s.loading = false
		if s.view.Refresh != nil && s.view.Refresh.Ref != nil {
			s.refreshAfterMutation()
		} else if s.view.Source != nil {
			s.reload()
		} else {
			h.Refresh()
		}
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
