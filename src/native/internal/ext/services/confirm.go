package services

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ConfirmWait bounds how long a destructive call waits for the user. The palette shows the question meanwhile; an
// answer that does not come in time cancels the call.
const ConfirmWait = 60 * time.Second

// Question is what the palette asks before a destructive call runs: the extension that asked and the action.
type Question struct {
	Extension string
	Action    string
}

// ConfirmFunc shows a question in the palette and returns the user's answer. It blocks until the user answers or
// ctx ends. The host owns it: an extension cannot answer its own question.
type ConfirmFunc func(ctx context.Context, question Question) (bool, error)

// waitMarkKey carries the waitMark of one service call through its context.
type waitMarkKey struct{}

// waitMark records that a call waited for the user. The supervisor restarts the request's hard budget after such a
// call, so the time a person takes to answer is not charged to the extension.
type waitMark struct {
	waited bool
}

// confirmDestructive asks the user before a destructive call runs. It refuses when no confirmation port is wired,
// and when the user does not confirm. The wait does not use the request's context: the request budget is far shorter
// than a person's answer.
func (h *Handler) confirmDestructive(ctx context.Context, action string) error {
	if h.deps.Confirm == nil {
		return unavailable("this host cannot ask for confirmation, so %s did not run", action)
	}
	if mark, ok := ctx.Value(waitMarkKey{}).(*waitMark); ok {
		mark.waited = true
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), ConfirmWait)
	defer cancel()
	confirmed, err := h.deps.Confirm(waitCtx, Question{Extension: h.manifest.Identity.Title, Action: action})
	if err != nil {
		return serviceError{failure: api.Failure{Code: api.CodeCancelled, Message: action + " was not confirmed in time"}, cause: api.ErrPermission}
	}
	if !confirmed {
		return serviceError{failure: api.Failure{Code: api.CodePermission, Message: action + " was cancelled"}, cause: api.ErrPermission}
	}
	return nil
}
