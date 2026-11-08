package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// runFollowUp runs the action a result names: a confirmation first when it asks for one, a run action through the
// extension (whose own result is applied the same way, up to maxFollowUps deep), or a declarative action on the host.
func (s *Session) runFollowUp(h search.Host, extID string, action api.Action, identity any, identityBound bool, depth int) {
	if depth > maxFollowUps {
		h.Toast("This action asked for too many follow-up actions")
		return
	}
	if !identityBound && extID == s.extID {
		identity, identityBound = s.installationIdentity, true
	}
	if action.Kind == api.ActionRun && !identityBound {
		h.Toast(unavailableMessage)
		return
	}
	if action.Confirm != nil {
		r := route{extID: extID, action: action, viewIntent: true, identity: identity, identityBound: identityBound}
		if action.Kind == api.ActionToggleSetting && extID == s.extID {
			r.identity = s.installationIdentity
		}
		h.Push(s.confirmPage(r))
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
	in := api.Invocation{Ref: *action.Ref, RequireIdentity: true, ExpectedIdentity: identity}
	s.deps.Invoke(extID, in, func(result api.Result, failure *api.Failure) {
		if !result.OriginBound {
			result.OriginIdentity, result.OriginBound = identity, true
		}
		if result.DataChanged {
			s.revalidateMutation(extID, result.OriginIdentity)
		}
		if extID == s.extID && !s.currentInstallation() {
			return
		}
		s.finishAt(h, extID, action.Title, result, failure, depth)
	})
}
