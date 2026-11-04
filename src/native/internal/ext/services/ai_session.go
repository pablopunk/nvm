package services

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// sessionAsk answers a call under a session. The session's earlier turns go with the prompt, and the turn is kept
// once the reply is complete.
func (h *Handler) sessionAsk(ctx context.Context, args protocol.AIAskArgs) (string, error) {
	session, err := h.ai.session(args.Session, args.Options)
	if err != nil {
		return "", err
	}
	completed, err := h.deps.AI.Complete(ctx, extai.Call{Options: session.optionsFor(args.Options), Prompt: args.Prompt, History: session.history()})
	if err != nil {
		return "", aiFailure(err)
	}
	session.record(completed.Prompt, completed.Text)
	return completed.Text, nil
}
