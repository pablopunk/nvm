package chat

import "github.com/pablopunk/nvm/native/internal/ai/agent"

// consume reads the events of a turn off its loop and hands each one to the UI thread, in order. When the stream ends it
// hands the outcome to the UI thread to settle. A stream that ends without a terminal event is an interrupted turn.
func (s *Service) consume(id string, r *run) {
	outcome := failed(agent.ErrTurnInterrupted)
	for event := range r.turn.Events() {
		outcome = outcomeOf(event, outcome)
		s.opts.Dispatch(func() { s.apply(id, r, event) })
	}
	s.opts.Dispatch(func() { s.settle(id, r, outcome) })
}

// apply writes one event of a turn into the chat. It runs on the UI thread.
func (s *Service) apply(id string, r *run, event agent.Event) {
	switch event.Kind {
	case agent.EventStep:
		if event.Step > 1 {
			s.beginReply(id)
		}
	case agent.EventDelta:
		s.appendText(id, event.Text)
	case agent.EventCalls:
		s.endReply(id, event.Calls)
	case agent.EventToolStart:
		s.startToolRun(id, event.Call)
	case agent.EventApproval:
		r.pending = event.Call.ID
		s.setToolStatus(id, event.Call.ID, StatusAwaiting, "")
	case agent.EventToolEnd:
		if r.pending == event.Call.ID {
			r.pending = ""
		}
		s.endToolRun(id, event)
	}
}

// appendText adds a piece of reply text to the streaming message of the chat.
func (s *Service) appendText(id, text string) {
	_ = s.mutate(id, func(c *Chat) {
		if index := streamingIndex(c); index >= 0 {
			c.Messages[index].Text += text
		}
	})
}
