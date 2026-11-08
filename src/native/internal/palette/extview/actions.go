package extview

import (
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
)

// handle names an ActionRun the palette may run. Extension code never sees it, and the palette resolves it only
// through the session that issued it, so a row cannot run an action the view did not offer.
type handle string

// route is what a handle stands for: the extension that answers and the action it declared.
type route struct {
	extID         string
	action        api.Action
	command       bool
	viewIntent    bool
	initialView   *api.View
	identity      any
	identityBound bool
	depth         int
}

// ActionFor returns the palette action of one wire action of extension extID, for rows outside a view such as the
// root commands. Run actions go through the host, and declarative actions are host operations.
func ActionFor(deps Deps, extID string, action api.Action) search.Action {
	s := NewSession(deps, extID, api.View{})
	s.viewIntent = false
	return s.action(action, "")
}

// CommandActionFor maps the primary action of a root command to an action that invokes through InvokeCommand.
func CommandActionFor(deps Deps, extID string, action api.Action, initial ...*api.View) search.Action {
	s := NewSession(deps, extID, api.View{})
	s.viewIntent = false
	var view *api.View
	if len(initial) != 0 {
		view = initial[0]
	}
	return s.commandAction(action, view)
}

func (s *Session) commandAction(action api.Action, initial *api.View) search.Action {
	mapped := s.withShortcut(s.actionBody(action, ""), action.Shortcut)
	mapped.MenuOnly = action.Preview
	mapped.Subtitle = action.Subtitle
	if action.Kind == api.ActionRun && action.Ref != nil {
		mapped.Dismiss = runDismissal(action)
		token := s.issue("", route{extID: s.extID, action: action, command: true, initialView: initial})
		mapped.Run = func(h search.Host) { s.runHandle(h, token) }
	}
	return mapped
}

// actionAt maps the index-th action of an item to a palette action, keyed by the item so handles stay stable across
// searches.
func (s *Session) actionAt(itemID string, index int, action api.Action) search.Action {
	return s.action(action, itemID+"/"+strconv.Itoa(index))
}

// action maps one wire action to a palette action, with the accelerator the action names. key names the row that owns
// the action, for run handles.
func (s *Session) action(action api.Action, key string) search.Action {
	mapped := s.withShortcut(s.actionBody(action, key), action.Shortcut)
	mapped.MenuOnly = action.Preview
	mapped.Subtitle = action.Subtitle
	if action.Kind != api.ActionRun && s.deps.AdmitAction != nil {
		run := mapped.Run
		mapped.Run = func(h search.Host) {
			valid, err := s.deps.AdmitAction(s.extID, s.installationIdentity)
			if err != nil || valid == nil || !valid() {
				h.Toast(unavailableMessage)
				return
			}
			run(h)
		}
	}
	return mapped
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
	return s.actionBodyFor(route{extID: s.extID, action: action, identity: s.installationIdentity, identityBound: true, viewIntent: s.viewIntent}, key)
}

func (s *Session) actionBodyFor(r route, key string) search.Action {
	action := r.action
	title := orDefault(action.Title, defaultTitle(action.Kind))
	switch action.Kind {
	case api.ActionRun:
		if action.Ref == nil {
			return search.Action{Title: title, Run: func(h search.Host) { h.Toast(unavailableMessage) }}
		}
		token := s.issue(key, r)
		return search.Action{Title: title, Dismiss: runDismissal(action), Run: func(h search.Host) { s.runHandle(h, token) }}
	case api.ActionOpenURL, api.ActionCopyText, api.ActionCopyImage:
		return s.hostOperationFor(r, title)
	case api.ActionPasteText, api.ActionPaste, api.ActionTypeText:
		return s.pasteOperationFor(r, title)
	case api.ActionPushView:
		if action.View == nil {
			break
		}
		view := *action.View
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			child := s.navigationChild(r.extID, view, api.NavPush)
			if r.identityBound {
				child.installationIdentity = r.identity
			}
			h.Push(child.Open(h))
		}}
	case api.ActionToggleSetting:
		if s.deps.ToggleSetting == nil || strings.TrimSpace(action.Setting) == "" {
			break
		}
		token := s.issue(key, r)
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.runHandle(h, token) }}
	case api.ActionReplaceView, api.ActionRootView:
		return s.hostOperationFor(r, title)
	case api.ActionRefreshView:
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if s.dismissed {
				return
			}
			if (s.view.Kind != api.ViewList && s.view.Kind != api.ViewGrid) || s.view.Search != nil {
				h.Toast(unavailableMessage)
				return
			}
			if s.view.Source != nil && s.view.Refresh == nil {
				s.reload()
				s.refreshHost()
				return
			}
			if s.view.Source != nil || s.view.Refresh == nil || s.view.Refresh.Ref == nil || strings.TrimSpace(s.view.Refresh.Ref.ID) == "" {
				h.Toast(unavailableMessage)
				return
			}
			s.manualRefreshHost = h
			s.refreshNow()
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
		return s.hostOperationFor(r, title)
	case api.ActionUpdate:
		if s.deps.RunUpdate == nil {
			break
		}
		op := action.Op
		return search.Action{Title: title, Dismiss: runDismissal(action), Run: func(h search.Host) { s.runUpdate(h, op) }}
	case api.ActionOpenPath, api.ActionRevealPath, api.ActionSetQuery, api.ActionQuickLook, api.ActionOpenWith, api.ActionTrash, api.ActionWindow:
		return s.hostOperationFor(r, title)
	case api.ActionAccount:
		if action.Op != api.AccountSignIn || s.deps.SignIn == nil {
			break
		}
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.deps.SignIn(h) }}
	case api.ActionHost:
		op := action.Op
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			if s.deps.HostOperation == nil {
				h.Toast(unavailableMessage)
				return
			}
			s.deps.HostOperation(h, r.extID, op)
		}}
	case api.ActionShortcut:
		if s.deps.ShortcutAction == nil {
			break
		}
		op, target := action.Op, action.Target
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.deps.ShortcutAction(h, op, target) }}
	case api.ActionChat:
		if action.Chat == nil || s.deps.Chats == nil {
			break
		}
		return s.chatOperation(action, title, r)
	case api.ActionBuilder:
		if action.Builder == nil || s.deps.Chats == nil || !api.IsBuiltInID(r.extID) {
			break
		}
		return s.builderOperation(action, title, r)
	case api.ActionRecordPaletteShortcut:
		if s.deps.RecordPaletteShortcut == nil {
			break
		}
		return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { s.deps.RecordPaletteShortcut(h) }}
	}
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) { h.Toast(unavailableMessage) }}
}

// pasteOperationFor sends a paste or typing action to the app in front. The palette hides first, and the services of the
// extension perform it, so the capabilities the extension declared apply. The HUD reports the outcome.
func (s *Session) pasteOperationFor(r route, title string) search.Action {
	action := r.action
	dismiss := search.DismissAuto
	if action.Paste != nil && action.Paste.KeepPaletteOpen {
		dismiss = search.DismissNever
	}
	return search.Action{Title: title, Dismiss: dismiss, Run: func(h search.Host) {
		if s.deps.PasteBound != nil && r.identityBound {
			s.deps.PasteBound(h, r.extID, r.identity, action)
			return
		}
		if s.deps.Paste == nil {
			h.Toast(unavailableMessage)
			return
		}
		s.deps.Paste(h, r.extID, action)
	}}
}

// hostOperation is a declarative action the host performs itself. It runs through the route, so a Confirmation asks
// first, as it does for a run action.
func (s *Session) hostOperationFor(r route, title string) search.Action {
	return search.Action{Title: title, Dismiss: search.DismissNever, Run: func(h search.Host) {
		s.runRoute(h, r)
	}}
}

const unavailableMessage = "This action is not available yet"

func (s *Session) issue(key string, r route) handle {
	s.guard.Lock()
	defer s.guard.Unlock()
	token := handle(s.extID + "#" + key)
	s.handles[token] = r
	return token
}

// runHandle runs the action behind token, after its confirmation if it has one.
func (s *Session) runHandle(h search.Host, token handle) {
	s.guard.Lock()
	r, ok := s.handles[token]
	s.guard.Unlock()
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
		s.runFollowUp(h, extID, *result.Next, result.OriginIdentity, result.OriginBound, depth+1)
	case result.View != nil:
		switch result.Navigation {
		case api.NavRoot:
			h.Root()
		case api.NavReplace:
			h.Pop()
		}
		child := s.resultChild(extID, result)
		h.Push(child.Open(h))
	case result.Navigation == api.NavPop:
		h.Pop()
	case result.Patch != nil:
		s.Apply(*result.Patch)
		h.Refresh()
	default:
		h.Hide()
	}
}

func (s *Session) navigationChild(extID string, view api.View, navigation api.NavigationMode) *Session {
	child := NewSession(s.deps, extID, view)
	if extID == s.extID {
		child.installationIdentity = s.installationIdentity
	}
	switch navigation {
	case api.NavReplace:
		child.parent = s.parent
	case api.NavRoot:
	default:
		child.parent = s
	}
	return child
}

// resultChild binds a returned view to the runtime that produced it.
func (s *Session) resultChild(extID string, result api.Result) *Session {
	child := s.navigationChild(extID, *result.View, result.Navigation)
	if result.OriginBound {
		child.installationIdentity = result.OriginIdentity
	}
	return child
}

// confirmPage asks before a destructive action runs: its first row runs it, its second cancels. Cancel and Escape leave
// the page without running anything, and a confirmed action replaces the confirmation with its own result.
func (s *Session) confirmPage(r route) *palette.View {
	confirmation := r.action.Confirm
	title := orDefault(confirmation.Title, orDefault(r.action.Title, "Confirm"))
	label := orDefault(confirmation.ConfirmLabel, "Confirm")
	cancel := orDefault(confirmation.CancelLabel, "Cancel")
	tone := ""
	if confirmation.Destructive || string(r.action.Style) == "destructive" {
		tone = "red"
	}
	return &palette.View{
		Title: "Confirm", Placeholder: title,
		Search: func(string) []search.Item {
			return []search.Item{
				{Title: label, Subtitle: confirmation.Message, Icon: "check", Tone: tone, Actions: []search.Action{
					{Title: "Confirm", Dismiss: search.DismissNever, Run: func(h search.Host) {
						h.Pop()
						confirmed := r
						confirmed.action = withoutConfirmation(r.action)
						s.invoke(h, confirmed)
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
	s.deps.Invoke(s.extID, s.bindViewInvocation(s.extID, in), func(result api.Result, failure *api.Failure) {
		if result.DataChanged {
			s.revalidateMutation(s.extID, result.OriginIdentity)
		}
		if !s.currentInstallation() {
			return
		}
		if afterRun != nil {
			afterRun(failure != nil)
		}
		s.finish(h, s.extID, "", result, failure)
	})
}
