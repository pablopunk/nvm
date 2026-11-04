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
	return s.withShortcut(s.actionBody(action, key), action.Shortcut)
}

// withShortcut gives a palette action the keys of an accelerator, and the label the palette shows for them. An
// accelerator that does not parse is ignored.
func (s *Session) withShortcut(action search.Action, accelerator string) search.Action {
	if mods, key, ok := parseShortcut(accelerator); ok {
		action.Mods, action.Key, action.Label = mods, key, shortcutLabel(accelerator, s.hyperKey())
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
		return search.Action{Title: title, Dismiss: runDismissal(action), Run: func(h search.Host) { s.runHandle(h, token) }}
	case api.ActionOpenURL:
		url := action.URL
		return search.Action{Title: title, Run: func(h search.Host) { h.Hide(); s.deps.OpenURL(url) }}
	case api.ActionCopyText:
		text := action.Text
		return search.Action{Title: title, Run: func(h search.Host) { s.deps.CopyText(h, text) }}
	case api.ActionPasteText, api.ActionPaste, api.ActionTypeText:
		return s.pasteOperation(action, title)
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
	case api.ActionCamera:
		op := action.Op
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.operateCamera(h, op) }}
	case api.ActionSystem:
		if pane := action.Setting; action.Op == "open-system-settings" && s.deps.OpenSettingsPane != nil {
			return search.Action{Title: title, Run: func(h search.Host) { h.Hide(); s.deps.OpenSettingsPane(pane) }}
		}
		return s.hostOperation(action, title)
	case api.ActionOpenPath, api.ActionRevealPath, api.ActionSetQuery:
		return s.hostOperation(action, title)
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Toast(unavailableMessage) }}
}

// pasteOperation sends a paste or typing action to the app in front. The palette hides first, and the services of the
// extension perform it, so the capabilities the extension declared apply. The HUD reports the outcome.
func (s *Session) pasteOperation(action api.Action, title string) search.Action {
	return search.Action{Title: title, Run: func(h search.Host) {
		if s.deps.Paste == nil {
			h.Toast(unavailableMessage)
			return
		}
		s.deps.Paste(h, s.extID, action)
	}}
}

// hostOperation is a declarative action the host performs itself. It runs through the route, so a Confirmation asks
// first, as it does for a run action.
func (s *Session) hostOperation(action api.Action, title string) search.Action {
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		s.runRoute(h, route{extID: s.extID, action: action})
	}}
}

const unavailableMessage = "This action is not available yet"

func defaultTitle(kind api.ActionKind) string {
	switch kind {
	case api.ActionPaste, api.ActionPasteText:
		return "Paste"
	case api.ActionTypeText:
		return "Type"
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
// A declarative action is performed by the host instead, with no extension call.
func (s *Session) invoke(h search.Host, r route) {
	if r.action.Kind != api.ActionRun {
		s.runDeclared(h, r.action)
		return
	}
	if s.deps.Invoke == nil || r.action.Ref == nil {
		h.Toast(unavailableMessage)
		return
	}
	in := api.Invocation{Ref: *r.action.Ref}
	s.deps.Invoke(r.extID, in, func(result api.Result, failure *api.Failure) {
		s.finish(h, r.extID, r.action.Title, result, failure)
	})
}

// maxFollowUps bounds the chain of follow-up actions one run may start, so a handler that answers with itself stops.
const maxFollowUps = 4

// finish applies a result on the UI thread. A failure stays on screen under the search field. A follow-up action runs
// instead of the rest of the result, as Electron does. A view opens above the current page, replaces it, or becomes
// the root; a patch refreshes the visible rows; a pop goes back a page; an empty result closes the palette.
func (s *Session) finish(h search.Host, extID, title string, result api.Result, failure *api.Failure) {
	s.finishAt(h, extID, title, result, failure, 0)
}

// finishAt is finish after depth follow-up actions have already run in this chain. title titles the toast the result
// carries, as the action that produced it does.
func (s *Session) finishAt(h search.Host, extID, title string, result api.Result, failure *api.Failure, depth int) {
	s.showToast(h, extID, title, result.Toast)
	switch {
	case failure != nil:
		h.Toast(failure.Message)
	case result.Next != nil:
		s.runFollowUp(h, extID, *result.Next, depth+1)
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
					{Title: "Confirm", Dismiss: search.DismissNever, Run: func(h search.Host) {
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

// submit runs an action with a full invocation, such as a form's or an editor's submit, and applies its result like any
// other action. afterRun, when set, sees whether the run failed before the result is applied.
func (s *Session) submit(h search.Host, in api.Invocation, afterRun func(failed bool)) {
	if s.deps.Invoke == nil {
		h.Toast(unavailableMessage)
		return
	}
	s.deps.Invoke(s.extID, in, func(result api.Result, failure *api.Failure) {
		if afterRun != nil {
			afterRun(failure != nil)
		}
		s.finish(h, s.extID, "", result, failure)
	})
}

// runFollowUp runs the action a result names: a confirmation first when it asks for one, a run action through the
// extension (whose own result is applied the same way, up to maxFollowUps deep), or a declarative action on the host.
func (s *Session) runFollowUp(h search.Host, extID string, action api.Action, depth int) {
	if depth > maxFollowUps {
		h.Toast("This action asked for too many follow-up actions")
		return
	}
	if action.Confirm != nil {
		h.Push(s.confirmPage(route{extID: extID, action: action}))
		return
	}
	if action.Kind != api.ActionRun {
		s.action(action, "").Run(h)
		return
	}
	if action.Ref == nil || s.deps.Invoke == nil {
		h.Toast(unavailableMessage)
		return
	}
	s.deps.Invoke(extID, api.Invocation{Ref: *action.Ref}, func(result api.Result, failure *api.Failure) {
		s.finishAt(h, extID, action.Title, result, failure, depth)
	})
}

// showToast shows a result's toast as a passive indicator titled with the action's title. Without an indicator
// surface the message shows under the search field.
func (s *Session) showToast(h search.Host, extID, title string, toast *api.Toast) {
	if toast == nil || toast.Message == "" {
		return
	}
	if s.deps.Indicate == nil {
		h.Toast(toast.Message)
		return
	}
	s.deps.Indicate(extID, toastIndicator(title, *toast))
}

// toastIndicator is the indicator a toast draws. Errors stay four seconds and other tones 2.2 seconds, as the Electron
// host shows them. A toast with no tone reads as success.
func toastIndicator(title string, toast api.Toast) api.IndicatorInput {
	input := api.IndicatorInput{ID: "feedback", Title: orDefault(title, "Nevermind"), Subtitle: toast.Message, Status: api.IndicatorSuccess, DurationMs: 2200}
	if toast.Tone == "error" {
		input.Status = api.IndicatorError
		input.DurationMs = 4000
	}
	return input
}

// runDismissal says whether the palette stays open while an ActionRun handler works. An action that asks to be
// dismissed after its run closes the palette as the run is dispatched, as the 1.x auto dismissal did.
func runDismissal(action api.Action) search.Dismissal {
	if action.DismissAfterRun {
		return search.DismissAuto
	}
	return search.DismissNever
}
