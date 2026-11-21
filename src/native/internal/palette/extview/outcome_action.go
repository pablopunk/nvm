package extview

import "github.com/pablopunk/nvm/native/internal/search"

// beginOutcomeAction marks an asynchronous action pending on the host and returns its completion, which takes whether
// the run succeeded. The host learns the row only from a successful, still current run. A host that tracks no pending
// actions answers with fallback, which knows nothing of outcomes.
func beginOutcomeAction(h search.Host, fallback func() bool) func(ok bool) bool {
	if outcome, ok := h.(search.ActionOutcomeHost); ok {
		return outcome.BeginActionOutcome()
	}
	if loading, ok := h.(search.ActionLoadingHost); ok {
		current := loading.BeginAction()
		return func(bool) bool { return current() }
	}
	return func(bool) bool { return fallback() }
}
