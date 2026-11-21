package chat

import (
	"errors"
	"slices"
	"time"
	"unicode/utf8"

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

// reasoningFields are the log fields of a reasoning phase: the kind and tier of the chat, the step it belongs to and how
// long it ran. The reasoning text is never logged.
func reasoningFields(r *run, elapsed time.Duration) logx.F {
	return logx.F{"chat_kind": string(r.kind), "tier": string(r.tier), "step": r.steps, "duration_ms": elapsed.Milliseconds()}
}

// logTurnEnd writes the end of a turn to the app log at info: how it ended, how many model steps it used and how long
// it ran. It never carries the transcript.
func (s *Service) logTurnEnd(r *run, outcome Outcome) {
	s.opts.Logger.Info("chat turn ended", logx.F{
		"chat_kind":   string(r.kind),
		"tier":        string(r.tier),
		"ending":      string(outcome.Ending),
		"steps":       r.steps,
		"duration_ms": s.opts.Clock.Now().Sub(r.started).Milliseconds(),
	})
}

// truncateBytes returns the longest prefix of text that is at most n bytes and ends on a whole rune.
func truncateBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}
