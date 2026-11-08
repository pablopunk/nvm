package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// invoke starts the extension's action. The palette stays open while it runs; finish decides what the result does.
// A declarative action is performed by the host instead, with no extension call.
func (s *Session) invoke(h search.Host, r route) {
	if r.action.Kind == api.ActionToggleSetting {
		s.toggleDeclaredSetting(h, r)
		return
	}
	if r.action.Kind != api.ActionRun {
		s.runDeclared(h, r.action)
		return
	}
	if r.action.Ref == nil {
		h.Toast(unavailableMessage)
		return
	}
	in := api.Invocation{Ref: *r.action.Ref, Background: r.action.Background}
	if r.viewIntent {
		if r.identityBound {
			in.RequireIdentity, in.ExpectedIdentity = true, r.identity
		} else {
			in = s.bindViewInvocation(r.extID, in)
		}
	}
	var identity any
	var pending *Session
	var initialPending bool
	var snapshotVersion uint64
	var staleHashes map[[32]byte]bool
	var snapshotPaths map[[32]byte]bool
	if r.command && !in.Background && s.deps.CommandSnapshots != nil && s.deps.CommandSnapshotIdentity != nil {
		identity = s.deps.CommandSnapshotIdentity(r.extID)
		staleHashes = s.deps.CommandSnapshots.StaleHashes(identity, in.Ref)
		snapshotPaths = s.deps.CommandSnapshots.SnapshotPaths(identity, in.Ref)
		in.ProtectSnapshot = s.deps.CommandSnapshots.IsProtected(identity, in.Ref)
		if view, ok := s.deps.CommandSnapshots.Get(identity, in.Ref); ok {
			snapshotVersion = s.deps.CommandSnapshots.Version(identity, in.Ref)
			view.Refresh = nil
			pending = s.navigationChild(r.extID, view, "")
			pending.snapshotPending = true
			h.Push(pending.Open(h))
			in.OnSnapshotDenied = func(denial *api.SnapshotDenial) {
				s.onUI(func() {
					if !snapshotPathsCover(snapshotPaths, denial) {
						return
					}
					if s.deps.CommandSnapshots != nil {
						s.deps.CommandSnapshots.RemoveVersion(identity, in.Ref, snapshotVersion)
					}
					if pending.dismissed {
						return
					}
					pending.clearPrivateSnapshot(denial.Failure)
				})
			}
		}
	}
	if pending == nil && r.command && !in.Background && !r.action.DismissAfterRun && validInitialView(r.initialView) {
		view := api.View{Kind: r.initialView.Kind, Title: r.initialView.Title, Size: r.initialView.Size, Grid: r.initialView.Grid}
		view.IsLoading = true
		pending = s.navigationChild(r.extID, view, "")
		initialPending = true
		pending.snapshotPending = true
		h.Push(pending.Open(h))
	}
	in.AwaitInitialView = pending != nil
	current := func() bool { return r.action.DismissAfterRun || !s.dismissed }
	if loading, ok := h.(search.ActionLoadingHost); ok {
		current = loading.BeginAction()
	}
	invoke := s.deps.Invoke
	if r.command && s.deps.InvokeCommand != nil {
		invoke = s.deps.InvokeCommand
	} else if r.viewIntent && s.deps.InvokeViewAction != nil {
		invoke = s.deps.InvokeViewAction
	}
	if invoke == nil {
		if pending != nil {
			h.Pop()
		}
		h.Toast(unavailableMessage)
		return
	}
	done := func(result api.Result, failure *api.Failure) {
		if r.identityBound && !result.OriginBound {
			result.OriginIdentity, result.OriginBound = r.identity, true
		}
		if in.ProtectSnapshot {
			filterSnapshotDenied(&result, staleHashes)
		}
		if result.DataChanged {
			s.revalidateMutation(r.extID, result.OriginIdentity)
		}
		if (r.viewIntent && !s.currentInstallation()) || !current() || (s.dismissed && !r.action.DismissAfterRun) {
			return
		}
		if pending != nil {
			if pending.dismissed {
				return
			}
			if initialPending && failure != nil {
				pending.snapshotPending = false
				pending.loading, pending.failure = false, failure
				pending.refreshHost()
				return
			}
			if s.deps.CommandSnapshots != nil {
				s.deps.CommandSnapshots.RemoveVersion(identity, in.Ref, snapshotVersion)
			}
			popPendingPage(h)
		}
		if identity != nil && failure == nil && result.View != nil && result.View.Kind == api.ViewGrid && (result.Navigation == "" || result.Navigation == api.NavPush) && result.Next == nil && result.Toast == nil {
			child := s.navigationChild(r.extID, *result.View, result.Navigation)
			if result.OriginBound {
				identity = result.OriginIdentity
				child.installationIdentity = identity
			}
			child.snapshotRef, child.snapshotIdentity = &in.Ref, identity
			child.snapshotSourcePaths = verifiedSnapshotPaths(result.View.Items, result.SnapshotEvidence)
			h.Push(child.Open(h))
			return
		}
		s.finish(h, r.extID, r.action.Title, result, failure)
	}
	if pending != nil && s.deps.InvokeCommandContext != nil {
		s.deps.InvokeCommandContext(pending.workContext, r.extID, in, done)
	} else {
		invoke(r.extID, in, done)
	}
}

// validInitialView rejects extension-provided initial content before it can start any host operations.
func validInitialView(view *api.View) bool {
	return view != nil && (api.Command{ID: "initial", Title: "initial", InitialView: view}).Validate() == nil
}

// popPendingPage removes the loading page a command pushed before its result replaces it. A palette a shortcut opened
// would hide on a plain Pop, because that page is its entry view; swapping it for the result is not the user going back.
func popPendingPage(h search.Host) {
	if quiet, ok := h.(interface{ PopPage() }); ok {
		quiet.PopPage()
		return
	}
	h.Pop()
}
