package chat

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/agent"
)

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
	if event.Kind != agent.EventReasoning {
		s.endReasoning(r)
	}
	switch event.Kind {
	case agent.EventStep:
		r.steps = event.Step
		if event.Step > 1 {
			s.beginReply(id)
		}
	case agent.EventReasoning:
		s.beginReasoning(r)
		s.appendReasoning(id, event.Text)
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

// appendReasoning adds a piece of the model's reasoning to the streaming message, up to MaxReasoningBytes.
func (s *Service) appendReasoning(id, text string) {
	_ = s.mutate(id, func(c *Chat) {
		index := streamingIndex(c)
		if index < 0 {
			return
		}
		room := MaxReasoningBytes - len(c.Messages[index].Reasoning)
		if room > 0 {
			c.Messages[index].Reasoning += truncateBytes(text, room)
		}
	})
}

// beginReasoning opens the reasoning of a model turn and logs when it started.
func (s *Service) beginReasoning(r *run) {
	if !r.reasoningSince.IsZero() {
		return
	}
	r.reasoningSince = s.opts.Clock.Now()
	s.opts.Logger.Info("chat reasoning started", reasoningFields(r, 0))
}

// endReasoning closes an open reasoning and logs how long it ran. It runs when the model's reply moves on.
func (s *Service) endReasoning(r *run) {
	if r.reasoningSince.IsZero() {
		return
	}
	elapsed := s.opts.Clock.Now().Sub(r.reasoningSince)
	r.reasoningSince = time.Time{}
	s.opts.Logger.Info("chat reasoning ended", reasoningFields(r, elapsed))
}
