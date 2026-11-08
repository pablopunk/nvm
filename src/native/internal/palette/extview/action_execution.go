package extview

import "github.com/pablopunk/nvm/native/internal/search"

// actionExecution ties an asynchronous action to the actual page that started it.
type actionExecution struct {
	owner   *Session
	origin  route
	current func() bool
}

func (s *Session) beginExecution(h search.Host, origin route) *actionExecution {
	current := func() bool { return true }
	if loading, ok := h.(search.ActionLoadingHost); ok {
		current = loading.BeginAction()
	}
	return &actionExecution{owner: s, origin: origin, current: current}
}

func (e *actionExecution) mayPresent() bool {
	if !e.current() || (e.owner.dismissed && !e.origin.action.DismissAfterRun) || (e.origin.viewIntent && !e.owner.currentInstallation()) {
		return false
	}
	if e.origin.identityBound && e.owner.deps.CommandSnapshotIdentity != nil {
		current := e.owner.deps.CommandSnapshotIdentity(e.origin.extID)
		if (e.origin.identity != nil || current != nil) && !sameSnapshotIdentity(e.origin.identity, current) {
			return false
		}
	}
	return true
}
