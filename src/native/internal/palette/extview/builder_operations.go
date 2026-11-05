package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// builderOperation runs an AI builder action of a built-in extension on the host: the builder pages open on the
// palette's stack, and removing a chat is a toast when the store refuses it.
func (s *Session) builderOperation(action api.Action, title string) search.Action {
	payload, pages := *action.Builder, s.deps.Builder
	switch action.Op {
	case api.BuilderStart:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(pages.Start(payload.Prompt)) }}
	case api.BuilderTweak:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(pages.Tweak(payload.ExtensionID)) }}
	case api.BuilderList:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Push(pages.List()) }}
	case api.BuilderRemove:
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
