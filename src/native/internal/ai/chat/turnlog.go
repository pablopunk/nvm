package chat

import (
	"errors"
	"slices"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// logFailedTurn writes a turn that ended in a failure to the app log at warn: the chat's kind and tier, the notice it
// showed and how the backend failed. It never carries the transcript or the error's server text.
func (s *Service) logFailedTurn(chat Chat, outcome Outcome) {
	if outcome.Ending != EndFailed {
		return
	}
	fields := logx.F{
		"chat_kind": string(chat.Kind),
		"tier":      string(chat.Tier),
		"notice":    string(NoticeFor(outcome.Err, "").Kind),
	}
	var typed *backend.Error
	if errors.As(outcome.Err, &typed) {
		fields["kind"] = string(typed.Kind)
		fields["status"] = typed.Status
		fields["code"] = typed.Code
	} else if outcome.Err != nil {
		fields["error"] = outcome.Err.Error()
	}
	s.opts.Logger.Warn("chat turn failed", fields)
}

// settleCutOffTurns ends the turns a quit or a crash left unfinished. No loop runs them any more, so without this their
// replies would spin forever. It runs when the service opens, before any turn starts.
func (s *Service) settleCutOffTurns() {
	outcome := failed(agent.ErrTurnInterrupted)
	for _, chat := range slices.Clone(s.cache.chats) {
		if !cutOff(chat) {
			continue
		}
		_ = s.mutate(chat.ID, func(c *Chat) { finishTurn(c, outcome, s.opts.Clock.Now()) })
		s.logFailedTurn(chat, outcome)
	}
}

// cutOff reports whether the saved chat has a reply or a tool call that never ended.
func cutOff(chat Chat) bool {
	return Busy(chat) || slices.ContainsFunc(chat.Messages, func(m Message) bool {
		return m.Author == AuthorTool && toolStillRunning(m.Status)
	})
}
