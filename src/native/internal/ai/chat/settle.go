package chat

import (
	"slices"
	"time"
)

// settle ends a turn on the UI thread: it stops tracking the run, writes how the reply ended into the transcript,
// notes a spend-cap refusal on the account, and tells the host the turn is over.
func (s *Service) settle(id string, r *run, outcome Outcome) {
	if s.runs[id] == r {
		delete(s.runs, id)
	}
	if s.doomed[id] {
		_ = s.remove(id)
		return
	}
	if outcome.Ending == EndFinished && s.lastReplyEmpty(id) {
		outcome = failed(ErrEmptyReply)
	}
	_ = s.mutate(id, func(c *Chat) {
		finishTurn(c, outcome, s.opts.Clock.Now())
	})
	if chat, ok := s.Chat(id); ok {
		s.logFailedTurn(chat, outcome)
	}
	if outcome.Err != nil && s.opts.NoteError != nil {
		s.opts.NoteError(outcome.Err)
	}
	if s.opts.OnSettled != nil {
		s.opts.OnSettled(id, outcome)
	}
}

// lastReplyEmpty reports whether the streaming message of the chat has no text.
func (s *Service) lastReplyEmpty(id string) bool {
	chat, ok := s.Chat(id)
	if !ok {
		return false
	}
	index := streamingIndex(&chat)
	return index >= 0 && chat.Messages[index].Text == ""
}

// finishTurn writes the ending of a turn into the chat. Tool rows that never ended are cancelled. An empty reply that
// stopped or failed disappears; a reply with text keeps it, marked stopped, and a failure adds its notice after it.
func finishTurn(c *Chat, outcome Outcome, at time.Time) {
	endUnfinishedTools(c)
	partial := settleStreamingReply(c, outcome.Ending)
	if outcome.Ending == EndFailed {
		notice := NoticeFor(outcome.Err, partial)
		c.Messages = append(c.Messages, Message{Author: AuthorNotice, Text: notice.Message, Status: StatusComplete, Notice: &notice, At: at})
	}
}

// settleStreamingReply ends the reply that is still streaming, if any, and returns its text. An empty reply that stopped
// or failed disappears; a reply with text keeps it, marked stopped when the turn did not finish.
func settleStreamingReply(c *Chat, ending Ending) string {
	index := streamingIndex(c)
	if index < 0 {
		return ""
	}
	reply := &c.Messages[index]
	switch ending {
	case EndFinished:
		reply.Status = StatusComplete
	case EndStopped, EndFailed:
		if reply.Text == "" {
			c.Messages = slices.Delete(c.Messages, index, index+1)
			return ""
		}
		reply.Status = StatusStopped
	}
	return reply.Text
}

// streamingIndex returns the position of the reply that is still streaming, or -1.
func streamingIndex(c *Chat) int {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Status == StatusStreaming {
			return i
		}
	}
	return -1
}

// endUnfinishedTools cancels the tool rows the loop never reported as ended, such as a call a restart cut off.
func endUnfinishedTools(c *Chat) {
	for i := range c.Messages {
		if c.Messages[i].Author == AuthorTool && toolStillRunning(c.Messages[i].Status) {
			c.Messages[i].Status = StatusCancelled
		}
	}
}
