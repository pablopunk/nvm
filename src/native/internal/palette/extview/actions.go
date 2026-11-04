package extview

import (
	"strconv"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// handle names an ActionRun the palette may run. Extension code never sees it, and the palette resolves it only
// through the session that issued it, so a row cannot run an action the view did not offer.
type handle string

// route is what a handle stands for: the extension that answers and the action it declared.
type route struct {
	extID  string
	action api.Action
}

// ActionFor returns the palette action of one wire action of extension extID, for rows outside a view such as the
// root commands. Run actions go through the host, and declarative actions are host operations.
func ActionFor(deps Deps, extID string, action api.Action) search.Action {
	return NewSession(deps, extID, api.View{}).action(action, "")
}

// actionAt maps the index-th action of an item to a palette action, keyed by the item so handles stay stable across
// searches.
func (s *Session) actionAt(itemID string, index int, action api.Action) search.Action {
	return s.action(action, itemID+"/"+strconv.Itoa(index))
}

// action maps one wire action to a palette action, with the accelerator the action names. key names the row that owns
// the action, for run handles.
func (s *Session) action(action api.Action, key string) search.Action {
	return withShortcut(s.actionBody(action, key), action.Shortcut)
}

// withShortcut gives a palette action the keys of an accelerator. An accelerator that does not parse is ignored.
func withShortcut(action search.Action, accelerator string) search.Action {
	if mods, key, ok := parseShortcut(accelerator); ok {
		action.Mods, action.Key, action.Label = mods, key, shortcutLabel(accelerator)
	}
	return action
}

func (s *Session) actionBody(action api.Action, key string) search.Action {
	title := orDefault(action.Title, defaultTitle(action.Kind))
	switch action.Kind {
	case api.ActionRun:
		if action.Ref == nil {
			return search.Action{Title: title, Run: func(h search.Host) { h.Toast(unavailableMessage) }}
		}
		token := s.issue(key, route{extID: s.extID, action: action})
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.runHandle(h, token) }}
	case api.ActionOpenURL:
		url := action.URL
		return search.Action{Title: title, Run: func(h search.Host) { h.Hide(); s.deps.OpenURL(url) }}
	case api.ActionCopyText:
		text := action.Text
		return search.Action{Title: title, Run: func(h search.Host) { s.deps.CopyText(text); h.Hide() }}
	case api.ActionPushView:
		if action.View == nil {
			break
		}
		view := *action.View
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			h.Push(NewSession(s.deps, s.extID, view).Open(h))
		}}
	case api.ActionPop:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop() }}
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Toast(unavailableMessage) }}
}

const unavailableMessage = "This action is not available yet"

func defaultTitle(kind api.ActionKind) string {
	switch kind {
	case api.ActionOpenURL:
		return "Open"
	case api.ActionCopyText:
		return "Copy"
	case api.ActionPushView:
		return "Open"
	case api.ActionPop:
		return "Back"
	}
	return "Run"
}

func (s *Session) issue(key string, r route) handle {
	token := handle(s.extID + "#" + key)
	s.handles[token] = r
	return token
}

// runHandle runs the action behind token, after its confirmation if it has one.
func (s *Session) runHandle(h search.Host, token handle) {
	r, ok := s.handles[token]
	if !ok {
		h.Toast("This action expired; reopen the view")
		return
	}
	s.runRoute(h, r)
}

func (s *Session) runRoute(h search.Host, r route) {
	if r.action.Confirm != nil {
		h.Push(s.confirmPage(r))
		return
	}
	s.invoke(h, r)
}

// invoke starts the extension's action. The palette stays open while it runs; finish decides what the result does.
func (s *Session) invoke(h search.Host, r route) {
	if s.deps.Invoke == nil || r.action.Ref == nil {
		h.Toast(unavailableMessage)
		return
	}
	in := api.Invocation{Ref: *r.action.Ref}
	s.deps.Invoke(r.extID, in, func(result api.Result, failure *api.Failure) {
		s.finish(h, r.extID, result, failure)
	})
}

// finish applies a result on the UI thread. A failure stays on screen under the search field. A view opens above the
// current page, replaces it, or becomes the root; a patch refreshes the visible rows; a pop goes back a page; an empty
// result closes the palette.
func (s *Session) finish(h search.Host, extID string, result api.Result, failure *api.Failure) {
	switch {
	case failure != nil:
		h.Toast(failure.Message)
	case result.View != nil:
		switch result.Navigation {
		case api.NavRoot:
			h.Root()
		case api.NavReplace:
			h.Pop()
		}
		h.Push(NewSession(s.deps, extID, *result.View).Open(h))
	case result.Navigation == api.NavPop:
		h.Pop()
	case result.Patch != nil:
		s.Apply(*result.Patch)
		h.Refresh()
	default:
		h.Hide()
	}
}

// confirmPage asks before a destructive action runs: its first row runs it, its second cancels. Cancel and Escape leave
// the page without running anything, and a confirmed action replaces the confirmation with its own result.
func (s *Session) confirmPage(r route) *palette.View {
	confirmation := r.action.Confirm
	title := orDefault(confirmation.Title, orDefault(r.action.Title, "Confirm"))
	label := orDefault(confirmation.ConfirmLabel, "Confirm")
	cancel := orDefault(confirmation.CancelLabel, "Cancel")
	return &palette.View{
		Title: "Confirm", Placeholder: title,
		Search: func(string) []search.Item {
			return []search.Item{
				{Title: label, Subtitle: confirmation.Message, Icon: "check", Tone: "red", Actions: []search.Action{
					{Title: label, Dismiss: search.DismissNever, Run: func(h search.Host) {
						h.Pop()
						s.invoke(h, route{extID: r.extID, action: withoutConfirmation(r.action)})
					}},
				}},
				{Title: cancel, Icon: "x", Actions: []search.Action{
					{Title: cancel, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Pop() }},
				}},
			}
		},
	}
}

func withoutConfirmation(action api.Action) api.Action {
	action.Confirm = nil
	return action
}
