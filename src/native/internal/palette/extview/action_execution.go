package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

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

// backgroundFeedback survives page dismissal but never navigates or changes a newer palette.
func (e *actionExecution) backgroundFeedback(result api.Result, failure *api.Failure) {
	if !e.origin.action.Background || !e.origin.action.DismissAfterRun || e.owner.deps.Indicate == nil {
		return
	}
	if failure != nil {
		e.owner.deps.Indicate(e.origin.extID, toastIndicator(e.origin.action.Title, api.Toast{Message: failure.Message, Tone: "error"}))
		return
	}
	if result.Toast != nil && result.Toast.Message != "" {
		e.owner.deps.Indicate(e.origin.extID, toastIndicator(e.origin.action.Title, *result.Toast))
	}
}
